package workspace

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Google Sheets adapter — Create / Get / ReadRange / WriteRange against
// the v4 API. A1 notation everywhere; the UI builds it from a sheet +
// rect picker.

const gsheetsAPIBase = "https://sheets.googleapis.com/v4"

type Sheet struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	URL   string   `json:"url"`
	Tabs  []string `json:"tabs"`
}

type SheetEditor interface {
	Integration
	CreateSheet(ctx context.Context, token Token, title string) (Sheet, error)
	GetSheet(ctx context.Context, token Token, sheetID string) (Sheet, error)
	ReadRange(ctx context.Context, token Token, sheetID, a1Range string) ([][]string, error)
	WriteRange(ctx context.Context, token Token, sheetID, a1Range string, rows [][]string) error
}

type GSheetsAdapter struct {
	HTTPClient *http.Client
	APIBase    string
}

func NewGSheetsAdapter() *GSheetsAdapter { return &GSheetsAdapter{} }

func (a *GSheetsAdapter) Service() Service { return ServiceGoogle }

func (a *GSheetsAdapter) base() string {
	if a.APIBase != "" {
		return a.APIBase
	}
	return gsheetsAPIBase
}

type gsheetsRaw struct {
	SpreadsheetID string `json:"spreadsheetId"`
	Properties    struct {
		Title string `json:"title"`
	} `json:"properties"`
	Sheets []struct {
		Properties struct {
			Title string `json:"title"`
		} `json:"properties"`
	} `json:"sheets"`
}

func (g gsheetsRaw) toSheet() Sheet {
	tabs := make([]string, 0, len(g.Sheets))
	for _, s := range g.Sheets {
		tabs = append(tabs, s.Properties.Title)
	}
	return Sheet{
		ID:    g.SpreadsheetID,
		Title: g.Properties.Title,
		URL:   "https://docs.google.com/spreadsheets/d/" + g.SpreadsheetID + "/edit",
		Tabs:  tabs,
	}
}

func (a *GSheetsAdapter) CreateSheet(ctx context.Context, token Token, title string) (Sheet, error) {
	hc := googleClient(a.HTTPClient)
	body := map[string]any{"properties": map[string]any{"title": title}}
	var out gsheetsRaw
	if err := googleAPICall(ctx, hc, token, http.MethodPost, a.base()+"/spreadsheets", body, &out); err != nil {
		return Sheet{}, err
	}
	return out.toSheet(), nil
}

func (a *GSheetsAdapter) GetSheet(ctx context.Context, token Token, sheetID string) (Sheet, error) {
	if strings.TrimSpace(sheetID) == "" {
		return Sheet{}, fmt.Errorf("gsheets: sheetID required")
	}
	hc := googleClient(a.HTTPClient)
	// fields= keeps the payload small — without it Google returns the
	// full grid which can be megabytes.
	endpoint := a.base() + "/spreadsheets/" + url.PathEscape(sheetID) +
		"?fields=spreadsheetId,properties.title,sheets.properties.title"
	var out gsheetsRaw
	if err := googleAPICall(ctx, hc, token, http.MethodGet, endpoint, nil, &out); err != nil {
		return Sheet{}, err
	}
	return out.toSheet(), nil
}

type gsheetsValuesResp struct {
	Range          string     `json:"range"`
	MajorDimension string     `json:"majorDimension"`
	Values         [][]string `json:"values"`
}

// ReadRange returns values as the user sees them. FORMATTED_VALUE
// surfaces "$1,234.50" instead of "1234.5" for currency cells — that's
// what the UI typically wants to display.
func (a *GSheetsAdapter) ReadRange(ctx context.Context, token Token, sheetID, a1Range string) ([][]string, error) {
	if strings.TrimSpace(sheetID) == "" || strings.TrimSpace(a1Range) == "" {
		return nil, fmt.Errorf("gsheets: sheetID + a1Range required")
	}
	hc := googleClient(a.HTTPClient)
	// PathEscape the range — it can contain spaces, "!", and "$".
	endpoint := a.base() + "/spreadsheets/" + url.PathEscape(sheetID) +
		"/values/" + url.PathEscape(a1Range) +
		"?valueRenderOption=FORMATTED_VALUE"
	var resp gsheetsValuesResp
	if err := googleAPICall(ctx, hc, token, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Values, nil
}

// WriteRange overwrites the given range with rows. RAW means the input
// is taken literally — no formula interpretation, no number coercion;
// callers who want formula parsing should send valueInputOption=USER_ENTERED.
func (a *GSheetsAdapter) WriteRange(ctx context.Context, token Token, sheetID, a1Range string, rows [][]string) error {
	if strings.TrimSpace(sheetID) == "" || strings.TrimSpace(a1Range) == "" {
		return fmt.Errorf("gsheets: sheetID + a1Range required")
	}
	hc := googleClient(a.HTTPClient)
	endpoint := a.base() + "/spreadsheets/" + url.PathEscape(sheetID) +
		"/values/" + url.PathEscape(a1Range) + "?valueInputOption=RAW"
	body := map[string]any{
		"range":          a1Range,
		"majorDimension": "ROWS",
		"values":         rows,
	}
	return googleAPICall(ctx, hc, token, http.MethodPut, endpoint, body, nil)
}
