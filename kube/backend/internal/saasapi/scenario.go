package saasapi

import (
	"fmt"
	"strings"
)

// REST scenarios — multi-step load-test plans for the REST/HTTP test
// type. A scenario captures everything a SaaS worker needs to run:
//
//   - Optional pre-call to fetch a bearer token (or an API key applied
//     to every call instead).
//   - Up to 5 top-level endpoints, each potentially with chained
//     follow-up calls.
//   - Per-endpoint expectations: status code, exact-body match, and/or
//     a list of jsonpath assertions ("$.items length == 5", etc.).
//
// The desktop binary only builds and validates this shape; the actual
// executor lives in the SaaS platform. The desktop's local-runner path
// rejects scenarios with a clear "scenarios require Cloud runner"
// message — single-machine load testing stays single-request.

// MaxScenarioEndpoints is the user-facing cap on top-level endpoints.
// Chained calls don't count toward it.
const MaxScenarioEndpoints = 5

// DistLoadScenario is the multi-step plan posted to the SaaS API.
type DistLoadScenario struct {
	Auth      *RESTAuth      `json:"auth,omitempty"`
	Endpoints []RESTEndpoint `json:"endpoints"`
}

// RESTAuth picks how the worker authenticates outbound calls.
//
// Modes:
//   - ""/none   → no auth
//   - bearer    → call BearerAuthURL once per worker boot, extract a token
//                 via BearerTokenPath (gjson syntax, default "access_token"),
//                 then set `Authorization: Bearer <token>` on every endpoint.
//   - apiKey    → set APIKeyHeader (default "X-API-Key") to APIKeyValue on
//                 every endpoint. No pre-call.
type RESTAuth struct {
	Mode             string            `json:"mode"`
	BearerAuthURL    string            `json:"bearerAuthUrl,omitempty"`
	BearerMethod     string            `json:"bearerMethod,omitempty"`
	BearerBody       string            `json:"bearerBody,omitempty"`
	BearerHeaders    map[string]string `json:"bearerHeaders,omitempty"`
	BearerTokenPath  string            `json:"bearerTokenPath,omitempty"`
	APIKeyHeader     string            `json:"apiKeyHeader,omitempty"`
	APIKeyValue      string            `json:"apiKeyValue,omitempty"`
}

// RESTEndpoint is one HTTP request in the scenario. Variables
// captured by earlier steps (e.g. ${token} from the auth pre-call)
// are interpolated by the SaaS runner — desktop only serializes the
// raw strings.
type RESTEndpoint struct {
	Name    string            `json:"name,omitempty"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Expect  *RESTExpect       `json:"expect,omitempty"`
	Chain   []RESTEndpoint    `json:"chain,omitempty"`
}

// RESTExpect bundles the success criteria for one endpoint.
//   - Status: when non-zero, the response status must equal this value.
//             Zero means "any 2xx".
//   - BodyMatches: when non-empty, the response body must match this
//                  exact JSON (whitespace-normalized).
//   - FieldChecks: each runs against the response body via jsonpath.
type RESTExpect struct {
	Status      int          `json:"status,omitempty"`
	BodyMatches string       `json:"bodyMatches,omitempty"`
	FieldChecks []FieldCheck `json:"fieldChecks,omitempty"`
}

// FieldCheck is one jsonpath-based assertion.
//
// Kinds:
//   - exists  → path resolves to a non-null value
//   - equals  → value at path equals Value (string-compared)
//   - type    → value at path is the named type:
//               string | number | integer | boolean | array | object | null
//   - length  → array/string at path has the given length (int)
type FieldCheck struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Value any    `json:"value,omitempty"`
}

// Validate enforces the cap + shape rules that the SaaS runner depends
// on. Returns the first error encountered so the UI can highlight
// exactly one field at a time.
func (s *DistLoadScenario) Validate() error {
	if s == nil {
		return nil
	}
	if len(s.Endpoints) == 0 {
		return fmt.Errorf("scenario: at least one endpoint is required")
	}
	if len(s.Endpoints) > MaxScenarioEndpoints {
		return fmt.Errorf("scenario: %d endpoints exceeds the cap of %d", len(s.Endpoints), MaxScenarioEndpoints)
	}
	if s.Auth != nil {
		if err := s.Auth.Validate(); err != nil {
			return err
		}
	}
	for i := range s.Endpoints {
		if err := validateEndpoint(&s.Endpoints[i], fmt.Sprintf("endpoint[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

func (a *RESTAuth) Validate() error {
	mode := strings.ToLower(a.Mode)
	switch mode {
	case "", "none":
		return nil
	case "bearer":
		if strings.TrimSpace(a.BearerAuthURL) == "" {
			return fmt.Errorf("scenario auth: bearer mode requires bearerAuthUrl")
		}
		return nil
	case "apikey":
		if strings.TrimSpace(a.APIKeyValue) == "" {
			return fmt.Errorf("scenario auth: apiKey mode requires apiKeyValue")
		}
		return nil
	}
	return fmt.Errorf("scenario auth: unknown mode %q", a.Mode)
}

func validateEndpoint(e *RESTEndpoint, ctx string) error {
	if strings.TrimSpace(e.URL) == "" {
		return fmt.Errorf("%s: url is required", ctx)
	}
	if strings.TrimSpace(e.Method) == "" {
		return fmt.Errorf("%s: method is required", ctx)
	}
	if e.Expect != nil {
		for j, c := range e.Expect.FieldChecks {
			if err := validateFieldCheck(c, fmt.Sprintf("%s.expect.fieldChecks[%d]", ctx, j)); err != nil {
				return err
			}
		}
	}
	for j := range e.Chain {
		if err := validateEndpoint(&e.Chain[j], fmt.Sprintf("%s.chain[%d]", ctx, j)); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldCheck(c FieldCheck, ctx string) error {
	if strings.TrimSpace(c.Path) == "" {
		return fmt.Errorf("%s: path is required", ctx)
	}
	switch c.Kind {
	case "exists", "equals", "type", "length":
		return nil
	}
	return fmt.Errorf("%s: unknown kind %q (want exists|equals|type|length)", ctx, c.Kind)
}

// ScenarioStatus rides on DistLoadStatus.Scenario for scenario-shaped
// runs. The SaaS workers aggregate per-endpoint stats and stream them
// to the desktop so the dashboard can render a live table of which
// step is failing and how often.
type ScenarioStatus struct {
	Endpoints []EndpointStats `json:"endpoints"`
}

// EndpointStats is one row of the per-step report. Identifier is
// (Name, Method, URL) so the UI can group across workers without
// re-sending the scenario spec.
//
// The two "fails" counters separate transport failures (Status not in
// 2xx, network errors) from assertion failures (the request returned
// fine but Expect didn't match). They're tracked separately because
// they imply different actions: HTTPFails usually means the target
// service is degraded; AssertFails usually means the test was wrong
// or the contract changed.
type EndpointStats struct {
	Name        string  `json:"name,omitempty"`
	Method      string  `json:"method"`
	URL         string  `json:"url"`
	Executions  int64   `json:"executions"`
	Successes   int64   `json:"successes"`
	HTTPFails   int64   `json:"httpFails"`
	AssertFails int64   `json:"assertFails"`
	P50Ms       float64 `json:"p50Ms"`
	P95Ms       float64 `json:"p95Ms"`
	P99Ms       float64 `json:"p99Ms"`
	// LastFail is the most recent failure message (HTTP status + body
	// excerpt, or "$.items length expected 5, got 3") so the UI can
	// show *why* the latest run failed without the user opening logs.
	LastFail string `json:"lastFail,omitempty"`
}
