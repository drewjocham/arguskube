package saasapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScenario_Validate_Happy(t *testing.T) {
	s := &DistLoadScenario{
		Auth: &RESTAuth{Mode: "bearer", BearerAuthURL: "https://auth.example/token"},
		Endpoints: []RESTEndpoint{
			{
				Method: "GET", URL: "https://api.example/items",
				Expect: &RESTExpect{Status: 200, FieldChecks: []FieldCheck{
					{Path: "items.#", Kind: "length", Value: 5},
					{Path: "items.0", Kind: "type", Value: "integer"},
				}},
			},
		},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestScenario_Validate_Rejects(t *testing.T) {
	cases := []struct {
		name string
		in   *DistLoadScenario
		want string
	}{
		{
			name: "no endpoints",
			in:   &DistLoadScenario{Endpoints: nil},
			want: "at least one endpoint",
		},
		{
			name: "too many endpoints",
			in: func() *DistLoadScenario {
				eps := make([]RESTEndpoint, MaxScenarioEndpoints+1)
				for i := range eps {
					eps[i] = RESTEndpoint{Method: "GET", URL: "https://x"}
				}
				return &DistLoadScenario{Endpoints: eps}
			}(),
			want: "exceeds the cap",
		},
		{
			name: "missing url",
			in: &DistLoadScenario{Endpoints: []RESTEndpoint{
				{Method: "GET", URL: ""},
			}},
			want: "url is required",
		},
		{
			name: "missing method",
			in: &DistLoadScenario{Endpoints: []RESTEndpoint{
				{Method: "", URL: "https://x"},
			}},
			want: "method is required",
		},
		{
			name: "bad field-check kind",
			in: &DistLoadScenario{Endpoints: []RESTEndpoint{
				{Method: "GET", URL: "https://x", Expect: &RESTExpect{
					FieldChecks: []FieldCheck{{Path: "$", Kind: "regex"}},
				}},
			}},
			want: "unknown kind",
		},
		{
			name: "bearer without URL",
			in: &DistLoadScenario{
				Auth:      &RESTAuth{Mode: "bearer"},
				Endpoints: []RESTEndpoint{{Method: "GET", URL: "https://x"}},
			},
			want: "bearerAuthUrl",
		},
		{
			name: "apiKey without value",
			in: &DistLoadScenario{
				Auth:      &RESTAuth{Mode: "apiKey", APIKeyHeader: "X-API-Key"},
				Endpoints: []RESTEndpoint{{Method: "GET", URL: "https://x"}},
			},
			want: "apiKeyValue",
		},
		{
			name: "unknown auth mode",
			in: &DistLoadScenario{
				Auth:      &RESTAuth{Mode: "oauth2-pkce"},
				Endpoints: []RESTEndpoint{{Method: "GET", URL: "https://x"}},
			},
			want: "unknown mode",
		},
		{
			name: "chain depth validates too",
			in: &DistLoadScenario{Endpoints: []RESTEndpoint{
				{Method: "GET", URL: "https://x", Chain: []RESTEndpoint{
					{Method: "POST", URL: ""},
				}},
			}},
			want: "url is required",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.in.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want substring %q", err, c.want)
			}
		})
	}
}

func TestScenario_JSON_RoundTrip(t *testing.T) {
	// The SaaS contract is JSON; if a tag drifts we want the test to
	// fail before the wire goes out of sync.
	s := &DistLoadScenario{
		Auth: &RESTAuth{
			Mode: "bearer", BearerAuthURL: "https://x", BearerMethod: "POST",
			BearerHeaders: map[string]string{"Accept": "application/json"},
			BearerTokenPath: "data.access_token",
		},
		Endpoints: []RESTEndpoint{
			{
				Name: "list", Method: "GET", URL: "/v1/items",
				Expect: &RESTExpect{Status: 200},
				Chain: []RESTEndpoint{
					{Name: "detail", Method: "GET", URL: "/v1/items/${id}"},
				},
			},
		},
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got DistLoadScenario
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Auth.BearerAuthURL != "https://x" || got.Endpoints[0].Chain[0].URL != "/v1/items/${id}" {
		t.Fatalf("round-trip lost data: %+v", got)
	}
}
