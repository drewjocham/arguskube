package models_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/argus/api-gov/internal/models"
)

// TestEndpointJSONRoundTripPreservesSecurityShape catches the historical bug
// where Endpoint.Security was modeled as []string and silently dropped scopes.
// OpenAPI v3 says each requirement is a map of {schemeName: [scopes]} and
// the overall field is a list of such maps (the inner map AND-ed, the outer
// list OR-ed). We must round-trip without rearranging that shape.
func TestEndpointJSONRoundTripPreservesSecurityShape(t *testing.T) {
	original := models.Endpoint{
		ID:          "ep-1",
		SpecID:      "spec-1",
		Method:      "POST",
		Path:        "/users",
		OperationID: "createUser",
		Security: models.SecurityRequirements{
			{"oauth2": []string{"users:write", "users:read"}},
			{"apiKey": []string{}}, // alternative scheme, no scopes
		},
		Tags: []string{"users"},
	}

	raw, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded models.Endpoint
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Security) != 2 {
		t.Fatalf("expected 2 security requirements, got %d", len(decoded.Security))
	}
	if scopes := decoded.Security[0]["oauth2"]; len(scopes) != 2 || scopes[0] != "users:write" {
		t.Errorf("oauth2 scopes lost in round-trip: %v", scopes)
	}
	if _, ok := decoded.Security[1]["apiKey"]; !ok {
		t.Errorf("apiKey requirement lost in round-trip: %+v", decoded.Security[1])
	}
}

// TestEndpointEmbeddingHiddenFromJSON guards against accidental serialization
// of the pgvector embedding. Floats are large, leak nothing useful to the
// API consumer, and storing them on every response would bloat payloads.
func TestEndpointEmbeddingHiddenFromJSON(t *testing.T) {
	ep := models.Endpoint{
		ID:        "ep-1",
		Embedding: []float32{0.1, 0.2, 0.3},
	}
	raw, err := json.Marshal(&ep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "embedding") {
		t.Errorf("Embedding leaked into JSON: %s", raw)
	}
}

// TestEndpointOmitsZeroFields ensures the API surface stays compact — a
// blank Endpoint should not emit empty arrays/maps for Parameters,
// Responses, Tags, etc.
func TestEndpointOmitsZeroFields(t *testing.T) {
	ep := models.Endpoint{ID: "ep-1", Method: "GET", Path: "/x"}
	raw, err := json.Marshal(&ep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, banned := range []string{`"parameters":`, `"responses":`, `"security":`, `"tags":`, `"request_body":`} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("zero-valued field leaked into JSON (%s): %s", banned, raw)
		}
	}
}

// TestSecurityRequirementsValuerScannerRoundTrip exercises the JSONB
// adapters so we know a future swap to pgxpool keeps decoding the same.
func TestSecurityRequirementsValuerScannerRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   models.SecurityRequirements
	}{
		{
			name: "single scheme with scopes",
			in:   models.SecurityRequirements{{"oauth2": []string{"a", "b"}}},
		},
		{
			name: "alternatives (OR of AND)",
			in: models.SecurityRequirements{
				{"oauth2": []string{"read"}, "tenantId": []string{}},
				{"apiKey": []string{}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			driverVal, err := tc.in.Value()
			if err != nil {
				t.Fatalf("Value(): %v", err)
			}
			raw, ok := driverVal.([]byte)
			if !ok {
				t.Fatalf("Value() returned %T, want []byte", driverVal)
			}
			var got models.SecurityRequirements
			if err := got.Scan(raw); err != nil {
				t.Fatalf("Scan(): %v", err)
			}
			b1, _ := json.Marshal(tc.in)
			b2, _ := json.Marshal(got)
			if string(b1) != string(b2) {
				t.Errorf("round-trip mismatch:\n  in:  %s\n  out: %s", b1, b2)
			}
		})
	}
}

func TestSecurityRequirementsScanHandlesNullAndEmpty(t *testing.T) {
	cases := []struct {
		name string
		src  any
		want int // expected len
	}{
		{"nil source", nil, 0},
		{"json null bytes", []byte("null"), 0},
		{"empty bytes", []byte{}, 0},
		{"empty json array", []byte("[]"), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got models.SecurityRequirements
			if err := got.Scan(tc.src); err != nil {
				t.Fatalf("Scan(): %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("len=%d, want %d", len(got), tc.want)
			}
		})
	}
}

func TestSecurityRequirementsScanRejectsBadType(t *testing.T) {
	var got models.SecurityRequirements
	if err := got.Scan(42); err == nil {
		t.Error("expected error for unsupported source type")
	}
}

// TestSchemaJSONRoundTrip covers the deliberately-narrow JSON Schema surface.
// If we add fields (e.g. allOf) the round-trip should grow alongside.
func TestSchemaJSONRoundTrip(t *testing.T) {
	original := models.Schema{
		Type:     "object",
		Required: []string{"id", "name"},
		Properties: map[string]*models.Schema{
			"id":   {Type: "string", Format: "uuid"},
			"name": {Type: "string"},
			"tags": {Type: "array", Items: &models.Schema{Type: "string"}},
		},
		Example: map[string]any{"id": "abc", "name": "alice"},
	}
	raw, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded models.Schema
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "object" {
		t.Errorf("Type lost: %q", decoded.Type)
	}
	if len(decoded.Required) != 2 {
		t.Errorf("Required lost: %v", decoded.Required)
	}
	if decoded.Properties["tags"].Items.Type != "string" {
		t.Errorf("nested Items type lost: %+v", decoded.Properties["tags"])
	}
}

// TestAPISpecRoundTripPreservesContent ensures the raw spec bytes survive a
// JSON round-trip — base64-encoded on the wire but byte-identical after
// decode.
func TestAPISpecRoundTripPreservesContent(t *testing.T) {
	original := models.APISpec{
		ID:      "spec-1",
		Name:    "users-api",
		Version: "1.0.0",
		Format:  "json",
		Content: []byte(`{"openapi":"3.0.0","paths":{}}`),
	}
	raw, err := json.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded models.APISpec
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(decoded.Content) != string(original.Content) {
		t.Errorf("Content corrupted: in=%q out=%q", original.Content, decoded.Content)
	}
}
