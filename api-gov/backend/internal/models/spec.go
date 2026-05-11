package models

import "time"

type APISpec struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Version   string      `json:"version"`
	Content   []byte      `json:"content"`
	Format    string      `json:"format"`
	Endpoints []*Endpoint `json:"endpoints,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type Endpoint struct {
	ID          string              `json:"id"`
	SpecID      string              `json:"spec_id"`
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Summary     string              `json:"summary"`
	OperationID string              `json:"operation_id"`
	RequestBody *Schema             `json:"request_body,omitempty"`
	Responses   map[string]*Schema  `json:"responses,omitempty"`
	Parameters  []*Parameter        `json:"parameters,omitempty"`
	Security    []string            `json:"security,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Embedding   []float32           `json:"-"`
}

type Schema struct {
	Type       string             `json:"type,omitempty"`
	Required   []string           `json:"required,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
	Ref        string             `json:"ref,omitempty"`
	Format     string             `json:"format,omitempty"`
	Example    any                `json:"example,omitempty"`
}

type Parameter struct {
	Name     string  `json:"name"`
	In       string  `json:"in"`
	Required bool    `json:"required"`
	Schema   *Schema `json:"schema"`
}
