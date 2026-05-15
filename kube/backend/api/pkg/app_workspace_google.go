package pkg

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/argues/argus/internal/workspace"
)

// app_workspace_google.go — per-capability Wails methods backed by the
// unified Google connection (one OAuth grant, three adapters). Mirrors
// app_workspace_slack.go's resolve-then-call shape.
//
// Wails-method convention: NO context.Context in the signature. Wails
// maps every Go parameter to a JS argument, and ctx lives at a.appCtx().

// Lazy singletons — adapters hold only an HTTP client + base URL, so
// sharing across calls is fine and a build that never connects Google
// allocates nothing.
var (
	gdocsAdapterOnce   sync.Once
	gdocsAdapter       *workspace.GDocsAdapter
	gsheetsAdapterOnce sync.Once
	gsheetsAdapter     *workspace.GSheetsAdapter
	gtasksAdapterOnce  sync.Once
	gtasksAdapter      *workspace.GTasksAdapter
)

func getGDocsAdapter() *workspace.GDocsAdapter {
	gdocsAdapterOnce.Do(func() { gdocsAdapter = workspace.NewGDocsAdapter() })
	return gdocsAdapter
}
func getGSheetsAdapter() *workspace.GSheetsAdapter {
	gsheetsAdapterOnce.Do(func() { gsheetsAdapter = workspace.NewGSheetsAdapter() })
	return gsheetsAdapter
}
func getGTasksAdapter() *workspace.GTasksAdapter {
	gtasksAdapterOnce.Do(func() { gtasksAdapter = workspace.NewGTasksAdapter() })
	return gtasksAdapter
}

// resolveGoogleConnection finds + validates the caller's google
// connection and returns its (possibly-refreshed) token.
func (a *App) resolveGoogleConnection(sessionToken, connectionID string) (workspace.Token, error) {
	if !a.workspaceAvailable() {
		return workspace.Token{}, errors.New("workspace: not configured")
	}
	userID, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return workspace.Token{}, err
	}
	conns, err := a.workspace.List(a.appCtx(), userID)
	if err != nil {
		return workspace.Token{}, err
	}
	for _, c := range conns {
		if c.ID != connectionID {
			continue
		}
		if c.Service != workspace.ServiceGoogle {
			return workspace.Token{}, fmt.Errorf("workspace: connection %q is %s, not google", c.DisplayName, c.Service)
		}
		tok, err := a.workspace.Token(a.appCtx(), c.ID)
		if err != nil {
			return workspace.Token{}, fmt.Errorf("workspace: load token: %w", err)
		}
		return tok, nil
	}
	return workspace.Token{}, errors.New("workspace: connection not found for this user")
}

// --- Docs ----------------------------------------------------------------

func (a *App) CreateGoogleDoc(sessionToken, connectionID, title, body string) (workspace.Doc, error) {
	if strings.TrimSpace(title) == "" {
		return workspace.Doc{}, errors.New("gdocs: title is required")
	}
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Doc{}, err
	}
	return getGDocsAdapter().CreateDoc(a.appCtx(), tok, title, body)
}

func (a *App) ReadGoogleDoc(sessionToken, connectionID, docID string) (workspace.DocBody, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.DocBody{}, err
	}
	return getGDocsAdapter().GetDoc(a.appCtx(), tok, docID)
}

func (a *App) AppendGoogleDoc(sessionToken, connectionID, docID, text string) error {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return err
	}
	return getGDocsAdapter().AppendDoc(a.appCtx(), tok, docID, text)
}

// --- Sheets --------------------------------------------------------------

func (a *App) CreateGoogleSheet(sessionToken, connectionID, title string) (workspace.Sheet, error) {
	if strings.TrimSpace(title) == "" {
		return workspace.Sheet{}, errors.New("gsheets: title is required")
	}
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Sheet{}, err
	}
	return getGSheetsAdapter().CreateSheet(a.appCtx(), tok, title)
}

func (a *App) GetGoogleSheet(sessionToken, connectionID, sheetID string) (workspace.Sheet, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Sheet{}, err
	}
	return getGSheetsAdapter().GetSheet(a.appCtx(), tok, sheetID)
}

func (a *App) ReadGoogleSheetRange(sessionToken, connectionID, sheetID, a1Range string) ([][]string, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return nil, err
	}
	return getGSheetsAdapter().ReadRange(a.appCtx(), tok, sheetID, a1Range)
}

func (a *App) WriteGoogleSheetRange(sessionToken, connectionID, sheetID, a1Range string, rows [][]string) error {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return err
	}
	return getGSheetsAdapter().WriteRange(a.appCtx(), tok, sheetID, a1Range, rows)
}

// --- Tasks ---------------------------------------------------------------

func (a *App) ListGoogleTaskLists(sessionToken, connectionID string) ([]workspace.TaskList, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return nil, err
	}
	return getGTasksAdapter().ListTaskLists(a.appCtx(), tok)
}

func (a *App) ListGoogleTasks(sessionToken, connectionID, listID string) ([]workspace.Task, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return nil, err
	}
	return getGTasksAdapter().ListTasks(a.appCtx(), tok, listID)
}

func (a *App) CreateGoogleTask(sessionToken, connectionID, listID string, task workspace.Task) (workspace.Task, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Task{}, err
	}
	return getGTasksAdapter().CreateTask(a.appCtx(), tok, listID, task)
}

func (a *App) UpdateGoogleTask(sessionToken, connectionID, listID, taskID string, patch workspace.Task) (workspace.Task, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Task{}, err
	}
	return getGTasksAdapter().UpdateTask(a.appCtx(), tok, listID, taskID, patch)
}

func (a *App) DeleteGoogleTask(sessionToken, connectionID, listID, taskID string) error {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return err
	}
	return getGTasksAdapter().DeleteTask(a.appCtx(), tok, listID, taskID)
}
