package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// APISpec is a stored OpenAPI document plus its extracted endpoints. Content
// holds the original bytes (JSON or YAML) so the diff agent can compare new
// uploads against the previous version without lossy re-rendering.
type APISpec struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Version   string      `json:"version"`
	Content   []byte      `json:"content"`
	Format    string      `json:"format"` // "json" | "yaml"
	Endpoints []*Endpoint `json:"endpoints,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// Endpoint is one path-method pair extracted from the spec. Embedding is the
// pgvector representation used by the similarity-search endpoint; it is
// intentionally hidden from JSON because the float vector is not useful to
// API consumers and would bloat every response.
type Endpoint struct {
	ID          string               `json:"id"`
	SpecID      string               `json:"spec_id"`
	Method      string               `json:"method"`
	Path        string               `json:"path"`
	Summary     string               `json:"summary"`
	OperationID string               `json:"operation_id"`
	RequestBody *Schema              `json:"request_body,omitempty"`
	Responses   map[string]*Schema   `json:"responses,omitempty"`
	Parameters  []*Parameter         `json:"parameters,omitempty"`
	Security    SecurityRequirements `json:"security,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	Embedding   []float32            `json:"-"`
}

// SecurityRequirements models the OpenAPI v3 securityRequirement object —
// a list of `{schemeName: [scopes]}` maps where keys in a single map are
// AND-combined and entries in the outer list are OR-combined. Stored as
// JSONB so the nested shape survives a round-trip through Postgres.
type SecurityRequirements []map[string][]string

// Value implements driver.Valuer so pgx serializes the requirements as JSONB
// rather than trying to coerce them into a Postgres array type.
func (s SecurityRequirements) Value() (driver.Value, error) {
	if s == nil {
		// nil → SQL NULL. With the column's DEFAULT '[]' on INSERT and
		// preserved-as-null on UPDATE the absence of a security clause is
		// expressed clearly.
		return nil, nil
	}
	return json.Marshal(s)
}

// Scan implements sql.Scanner for reads from the JSONB column.
func (s *SecurityRequirements) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("SecurityRequirements.Scan: unsupported type %T", src)
	}
	if len(b) == 0 || string(b) == "null" {
		*s = nil
		return nil
	}
	return json.Unmarshal(b, s)
}

// Schema is a deliberately-narrow subset of JSON Schema sufficient for the
// drift-detection engine: type, required, properties, items, ref, format,
// example. Full coverage (allOf/oneOf/enum/pattern/...) is on the roadmap —
// when needed we will swap to github.com/getkin/kin-openapi instead of
// extending this struct.
type Schema struct {
	Type       string             `json:"type,omitempty"`
	Required   []string           `json:"required,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
	Ref        string             `json:"ref,omitempty"`
	Format     string             `json:"format,omitempty"`
	Example    any                `json:"example,omitempty"`
}

// Parameter is a path/query/header/cookie parameter on an endpoint.
type Parameter struct {
	Name     string  `json:"name"`
	In       string  `json:"in"` // "path" | "query" | "header" | "cookie"
	Required bool    `json:"required"`
	Schema   *Schema `json:"schema"`
}
