package workspace

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// Google Docs adapter — Create / Get / Append against the v1 API.
//
// We deliberately surface plain text only. Argus uses docs as a sink
// for run-summaries and notes; round-tripping rich-text styling would
// triple the surface and isn't needed for that use case.

const gdocsAPIBase = "https://docs.googleapis.com/v1"

// Doc is the lightweight document view returned to the UI.
type Doc struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`      // user-shareable link
	Modified int64  `json:"modified"` // unix; 0 when unknown
}

// DocBody is Doc + plain-text contents.
type DocBody struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// DocEditor is the post-OAuth surface for Google Docs operations.
type DocEditor interface {
	Integration
	CreateDoc(ctx context.Context, token Token, title, body string) (Doc, error)
	GetDoc(ctx context.Context, token Token, docID string) (DocBody, error)
	AppendDoc(ctx context.Context, token Token, docID, text string) error
}

type GDocsAdapter struct {
	HTTPClient *http.Client
	APIBase    string // tests override
}

func NewGDocsAdapter() *GDocsAdapter { return &GDocsAdapter{} }

func (a *GDocsAdapter) Service() Service { return ServiceGoogle }

func (a *GDocsAdapter) base() string {
	if a.APIBase != "" {
		return a.APIBase
	}
	return gdocsAPIBase
}

// gdocsRawDoc mirrors the subset of v1 Document we read. The full
// schema is enormous; we only need the title + a walkable body.
type gdocsRawDoc struct {
	DocumentID string `json:"documentId"`
	Title      string `json:"title"`
	Body       struct {
		Content []struct {
			Paragraph *struct {
				Elements []struct {
					TextRun *struct {
						Content string `json:"content"`
					} `json:"textRun"`
				} `json:"elements"`
			} `json:"paragraph"`
		} `json:"content"`
	} `json:"body"`
}

// CreateDoc creates a new document. When body is non-empty we follow
// up with a batchUpdate insertText so the user lands on a populated
// doc instead of a blank one.
func (a *GDocsAdapter) CreateDoc(ctx context.Context, token Token, title, body string) (Doc, error) {
	hc := googleClient(a.HTTPClient)
	var created gdocsRawDoc
	if err := googleAPICall(ctx, hc, token, http.MethodPost, a.base()+"/documents",
		map[string]string{"title": title}, &created); err != nil {
		return Doc{}, err
	}
	if body != "" {
		req := map[string]any{
			"requests": []map[string]any{{
				"insertText": map[string]any{
					"location": map[string]any{"index": 1}, // 1 is the first valid insertion offset.
					"text":     body,
				},
			}},
		}
		if err := googleAPICall(ctx, hc, token, http.MethodPost,
			a.base()+"/documents/"+created.DocumentID+":batchUpdate", req, nil); err != nil {
			return Doc{}, fmt.Errorf("gdocs: insert initial body: %w", err)
		}
	}
	return Doc{
		ID:    created.DocumentID,
		Title: created.Title,
		URL:   "https://docs.google.com/document/d/" + created.DocumentID + "/edit",
	}, nil
}

// GetDoc fetches the document and walks the content tree to extract
// plain text. Tables, lists, and inline images are ignored.
func (a *GDocsAdapter) GetDoc(ctx context.Context, token Token, docID string) (DocBody, error) {
	if strings.TrimSpace(docID) == "" {
		return DocBody{}, fmt.Errorf("gdocs: docID required")
	}
	hc := googleClient(a.HTTPClient)
	var d gdocsRawDoc
	if err := googleAPICall(ctx, hc, token, http.MethodGet,
		a.base()+"/documents/"+docID, nil, &d); err != nil {
		return DocBody{}, err
	}
	var sb strings.Builder
	for _, c := range d.Body.Content {
		if c.Paragraph == nil {
			continue
		}
		for _, e := range c.Paragraph.Elements {
			if e.TextRun != nil {
				sb.WriteString(e.TextRun.Content)
			}
		}
	}
	return DocBody{ID: d.DocumentID, Title: d.Title, Body: sb.String()}, nil
}

// AppendDoc appends text at the end of the document.
//
// endOfSegmentLocation with no segmentId targets the document body —
// that's the documented way to "insert at the end" without first
// fetching the doc to learn its length.
func (a *GDocsAdapter) AppendDoc(ctx context.Context, token Token, docID, text string) error {
	if strings.TrimSpace(docID) == "" {
		return fmt.Errorf("gdocs: docID required")
	}
	if text == "" {
		return nil
	}
	hc := googleClient(a.HTTPClient)
	req := map[string]any{
		"requests": []map[string]any{{
			"insertText": map[string]any{
				"endOfSegmentLocation": map[string]any{},
				"text":                 text,
			},
		}},
	}
	return googleAPICall(ctx, hc, token, http.MethodPost,
		a.base()+"/documents/"+docID+":batchUpdate", req, nil)
}
