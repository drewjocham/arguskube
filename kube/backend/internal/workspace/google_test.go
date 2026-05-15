package workspace

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoogleProvider_StartBuildsAuthURL(t *testing.T) {
	p := &GoogleProvider{
		ClientID:    "cid",
		RedirectURL: "https://argus.example/workspace/oauth/callback",
	}
	auth, err := p.Start(context.Background(), "u", "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !strings.HasPrefix(auth.URL, "https://accounts.google.com/o/oauth2/v2/auth?") {
		t.Fatalf("wrong base: %s", auth.URL)
	}
	u, err := url.Parse(auth.URL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	for _, want := range []string{
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/tasks",
		"https://www.googleapis.com/auth/userinfo.email",
	} {
		if !strings.Contains(q.Get("scope"), want) {
			t.Errorf("scope missing %q in %q", want, q.Get("scope"))
		}
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("expected S256 challenge method, got %q", q.Get("code_challenge_method"))
	}
	if q.Get("code_challenge") == "" {
		t.Errorf("missing code_challenge")
	}
	if q.Get("access_type") != "offline" {
		t.Errorf("missing access_type=offline")
	}
	if q.Get("prompt") != "consent" {
		t.Errorf("missing prompt=consent")
	}
	if q.Get("state") != auth.State {
		t.Errorf("state mismatch in URL vs return")
	}
	// Verifier should be retained inside the provider for Complete.
	p.flightMu.Lock()
	defer p.flightMu.Unlock()
	if _, ok := p.flights[auth.State]; !ok {
		t.Errorf("verifier not stored for state")
	}
}

func TestGoogleProvider_CompleteExchangesAndFetchesUserinfo(t *testing.T) {
	var gotTokenBody string
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotTokenBody = string(b)
		_, _ = w.Write([]byte(`{
			"access_token": "ya29.access",
			"refresh_token": "1//refresh",
			"expires_in": 3600,
			"scope": "openid email",
			"token_type": "Bearer"
		}`))
	}))
	defer tokenSrv.Close()
	userinfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer ya29.access" {
			t.Errorf("userinfo missing bearer: %q", got)
		}
		_, _ = w.Write([]byte(`{
			"sub": "12345",
			"email": "alice@corp.com",
			"name": "Alice",
			"picture": "https://example/a.png",
			"hd": "corp.com"
		}`))
	}))
	defer userinfoSrv.Close()

	p := &GoogleProvider{
		ClientID: "cid", ClientSecret: "csec",
		RedirectURL: "https://argus.example/cb",
		TokenURL:    tokenSrv.URL,
		UserinfoURL: userinfoSrv.URL,
	}
	auth, _ := p.Start(context.Background(), "u", "")
	res, err := p.Complete(context.Background(), auth.State, "test-code")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(gotTokenBody, "code=test-code") || !strings.Contains(gotTokenBody, "code_verifier=") {
		t.Errorf("token body missing code/verifier: %s", gotTokenBody)
	}
	if !strings.Contains(gotTokenBody, "grant_type=authorization_code") {
		t.Errorf("missing grant_type: %s", gotTokenBody)
	}
	if res.Email != "alice@corp.com" || res.DisplayName != "Alice" {
		t.Errorf("identity wrong: %+v", res)
	}
	if res.ExternalWorkspaceID != "corp.com" {
		t.Errorf("hd not surfaced: %q", res.ExternalWorkspaceID)
	}
	if res.Token.AccessToken != "ya29.access" || res.Token.RefreshToken != "1//refresh" {
		t.Errorf("token round-trip lost data: %+v", res.Token)
	}
	if res.Token.ExpiresAt.IsZero() || time.Until(res.Token.ExpiresAt) > time.Hour+time.Minute {
		t.Errorf("expires not set right: %v", res.Token.ExpiresAt)
	}
}

func TestGoogleProvider_CompleteRejectsUnknownState(t *testing.T) {
	p := &GoogleProvider{ClientID: "cid", ClientSecret: "csec", RedirectURL: "https://x/cb"}
	_, err := p.Complete(context.Background(), "never-issued", "code")
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown-state error, got %v", err)
	}
}

func TestGoogleProvider_RefreshKeepsPreviousRefreshTokenWhenEmpty(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// refresh response with NO refresh_token field — Google's typical
		// behaviour when not rotating.
		_, _ = w.Write([]byte(`{
			"access_token": "ya29.fresh",
			"expires_in": 3600,
			"scope": "openid",
			"token_type": "Bearer"
		}`))
	}))
	defer tokenSrv.Close()
	p := &GoogleProvider{
		ClientID: "cid", ClientSecret: "csec",
		RedirectURL: "https://x/cb", TokenURL: tokenSrv.URL,
	}
	fresh, err := p.Refresh(context.Background(), "1//old-refresh")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if fresh.AccessToken != "ya29.fresh" {
		t.Errorf("access wrong: %q", fresh.AccessToken)
	}
	if fresh.RefreshToken != "" {
		t.Errorf("expected empty refresh from response (manager preserves old), got %q", fresh.RefreshToken)
	}
}

func TestGoogleProvider_RefreshSurfacesError(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`))
	}))
	defer tokenSrv.Close()
	p := &GoogleProvider{
		ClientID: "cid", ClientSecret: "csec",
		RedirectURL: "https://x/cb", TokenURL: tokenSrv.URL,
	}
	_, err := p.Refresh(context.Background(), "1//dead")
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("expected invalid_grant in error, got %v", err)
	}
}

// --- Adapter tests -------------------------------------------------------

// gdocsTestHandler routes the three endpoints the adapter calls to
// small per-endpoint helpers. The split is here purely so each handler
// has low cognitive complexity (sonar's threshold is 15).
func gdocsTestHandler(t *testing.T, gotBatchUpdate *string) http.HandlerFunc {
	t.Helper()
	const docBody = `{
		"documentId":"D1","title":"Run notes",
		"body":{"content":[
			{"paragraph":{"elements":[{"textRun":{"content":"Hello "}}]}},
			{"paragraph":{"elements":[{"textRun":{"content":"world\n"}}]}}
		]}
	}`
	handleCreate := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"title":"Run notes"`) {
			t.Errorf("create body wrong: %s", b)
		}
		_, _ = w.Write([]byte(`{"documentId":"D1","title":"Run notes"}`))
	}
	handleBatchUpdate := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*gotBatchUpdate = string(b)
		_, _ = w.Write([]byte(`{}`))
	}
	handleGet := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(docBody))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/documents" && r.Method == http.MethodPost:
			handleCreate(w, r)
		case strings.HasSuffix(r.URL.Path, ":batchUpdate") && r.Method == http.MethodPost:
			handleBatchUpdate(w, r)
		case r.URL.Path == "/documents/D1" && r.Method == http.MethodGet:
			handleGet(w, r)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}
}

func TestGDocsAdapter_CreateAndGetAndAppend(t *testing.T) {
	var gotBatchUpdate string
	srv := httptest.NewServer(gdocsTestHandler(t, &gotBatchUpdate))
	defer srv.Close()
	a := &GDocsAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
	tok := Token{AccessToken: "ya29.x"}

	doc, err := a.CreateDoc(context.Background(), tok, "Run notes", "Initial body")
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}
	if doc.ID != "D1" || !strings.HasSuffix(doc.URL, "/document/d/D1/edit") {
		t.Errorf("doc surface wrong: %+v", doc)
	}
	if !strings.Contains(gotBatchUpdate, `"text":"Initial body"`) {
		t.Errorf("initial-body insertText missing: %s", gotBatchUpdate)
	}

	body, err := a.GetDoc(context.Background(), tok, "D1")
	if err != nil {
		t.Fatalf("GetDoc: %v", err)
	}
	if body.Body != "Hello world\n" {
		t.Errorf("text extract wrong: %q", body.Body)
	}

	if err := a.AppendDoc(context.Background(), tok, "D1", "appended"); err != nil {
		t.Fatalf("AppendDoc: %v", err)
	}
	if !strings.Contains(gotBatchUpdate, "endOfSegmentLocation") {
		t.Errorf("append didn't use endOfSegmentLocation: %s", gotBatchUpdate)
	}
}

func TestGDocsAdapter_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"PERMISSION_DENIED"}}`))
	}))
	defer srv.Close()
	a := &GDocsAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
	_, err := a.GetDoc(context.Background(), Token{AccessToken: "ya29.x"}, "D1")
	if err == nil || !strings.Contains(err.Error(), "PERMISSION_DENIED") {
		t.Fatalf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestGSheetsAdapter_ReadRangeEncodesA1(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"values":[["1","2"],["3","4"]]}`))
	}))
	defer srv.Close()
	a := &GSheetsAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
	rows, err := a.ReadRange(context.Background(), Token{AccessToken: "x"}, "S1", "Sheet1!A1:B2")
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if len(rows) != 2 || rows[0][0] != "1" {
		t.Fatalf("rows wrong: %+v", rows)
	}
	// The A1 range must appear in the path (Google accepts "!" unescaped
	// per RFC 3986 sub-delims). What matters is that we passed it as a
	// path component and not lost the sheet/tab prefix.
	if !strings.Contains(gotPath, "Sheet1") || !strings.Contains(gotPath, "A1:B2") {
		t.Errorf("A1 range missing from path: %s", gotPath)
	}
}

func TestGSheetsAdapter_WriteRangeRAW(t *testing.T) {
	var gotURL string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	a := &GSheetsAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
	err := a.WriteRange(context.Background(), Token{AccessToken: "x"}, "S1", "Sheet1!A1:B2",
		[][]string{{"=SUM(1,2)", "hello"}, {"3", "4"}})
	if err != nil {
		t.Fatalf("WriteRange: %v", err)
	}
	if !strings.Contains(gotURL, "valueInputOption=RAW") {
		t.Errorf("expected RAW input option: %s", gotURL)
	}
	if !strings.Contains(gotBody, `"=SUM(1,2)"`) {
		t.Errorf("body lost formula text: %s", gotBody)
	}
}

func TestGSheetsAdapter_CreateAndGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/spreadsheets" {
			_, _ = w.Write([]byte(`{"spreadsheetId":"S1","properties":{"title":"Q4"},"sheets":[{"properties":{"title":"Sheet1"}},{"properties":{"title":"Notes"}}]}`))
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/spreadsheets/S1") {
			_, _ = w.Write([]byte(`{"spreadsheetId":"S1","properties":{"title":"Q4"},"sheets":[{"properties":{"title":"Sheet1"}}]}`))
			return
		}
		t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	a := &GSheetsAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
	s, err := a.CreateSheet(context.Background(), Token{AccessToken: "x"}, "Q4")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	if s.ID != "S1" || s.Title != "Q4" || len(s.Tabs) != 2 {
		t.Errorf("create result wrong: %+v", s)
	}
	if !strings.HasSuffix(s.URL, "/spreadsheets/d/S1/edit") {
		t.Errorf("share URL wrong: %s", s.URL)
	}
	g, err := a.GetSheet(context.Background(), Token{AccessToken: "x"}, "S1")
	if err != nil || g.Title != "Q4" {
		t.Errorf("get wrong: %+v %v", g, err)
	}
}

// gtasksTestHandler routes the five tasks-API endpoints the adapter
// exercises. Same split-for-complexity rationale as gdocsTestHandler.
func gtasksTestHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	handleListLists := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"L1","title":"Inbox"}]}`))
	}
	handleListTasks := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"T1","title":"do","status":"needsAction"}]}`))
	}
	handleCreate := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var t1 Task
		_ = json.Unmarshal(body, &t1)
		if t1.Status != "needsAction" {
			t.Errorf("expected default status set, got %q", t1.Status)
		}
		_, _ = w.Write([]byte(`{"id":"T2","title":"` + t1.Title + `","status":"needsAction"}`))
	}
	handlePatch := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"T2","title":"renamed","status":"completed"}`))
	}
	handleDelete := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/users/@me/lists":
			handleListLists(w, r)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/lists/L1/tasks"):
			handleListTasks(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/lists/L1/tasks":
			handleCreate(w, r)
		case r.Method == http.MethodPatch && r.URL.Path == "/lists/L1/tasks/T2":
			handlePatch(w, r)
		case r.Method == http.MethodDelete && r.URL.Path == "/lists/L1/tasks/T2":
			handleDelete(w, r)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}
}

func TestGTasksAdapter_CRUD(t *testing.T) {
	srv := httptest.NewServer(gtasksTestHandler(t))
	defer srv.Close()
	a := &GTasksAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
	tok := Token{AccessToken: "x"}

	lists, err := a.ListTaskLists(context.Background(), tok)
	if err != nil || len(lists) != 1 || lists[0].ID != "L1" {
		t.Fatalf("ListTaskLists: %+v %v", lists, err)
	}
	tasks, err := a.ListTasks(context.Background(), tok, "L1")
	if err != nil || len(tasks) != 1 {
		t.Fatalf("ListTasks: %+v %v", tasks, err)
	}
	created, err := a.CreateTask(context.Background(), tok, "L1", Task{Title: "ship"})
	if err != nil || created.ID != "T2" {
		t.Fatalf("CreateTask: %+v %v", created, err)
	}
	updated, err := a.UpdateTask(context.Background(), tok, "L1", "T2", Task{Status: "completed"})
	if err != nil || updated.Status != "completed" {
		t.Fatalf("UpdateTask: %+v %v", updated, err)
	}
	if err := a.DeleteTask(context.Background(), tok, "L1", "T2"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
}

// --- Manager refresh integration ----------------------------------------

// fakeRefresher is a Provider+Refresher counting how many times Refresh
// is called, used to assert single-flight coalescing.
type fakeRefresher struct {
	svc          Service
	refreshCalls int32
	respToken    Token
	respErr      error
	block        chan struct{} // if non-nil, Refresh waits on it
}

func (f *fakeRefresher) Service() Service { return f.svc }
func (f *fakeRefresher) Start(_ context.Context, _, _ string) (AuthURL, error) {
	return AuthURL{}, nil
}
func (f *fakeRefresher) Complete(_ context.Context, _, _ string) (CompleteResult, error) {
	return CompleteResult{}, nil
}
func (f *fakeRefresher) Refresh(_ context.Context, _ string) (Token, error) {
	atomic.AddInt32(&f.refreshCalls, 1)
	if f.block != nil {
		<-f.block
	}
	return f.respToken, f.respErr
}

func TestManager_TokenRefreshesExpiredAndPersists(t *testing.T) {
	m := newManager(t)
	delete(m.providers, ServiceSlack)
	fake := &fakeRefresher{
		svc: ServiceGoogle,
		respToken: Token{
			AccessToken: "fresh-access",
			TokenType:   "Bearer",
			Scope:       "x",
			ExpiresAt:   time.Now().Add(time.Hour),
		},
	}
	m.providers[ServiceGoogle] = fake

	// Seed an expired connection.
	c, err := m.store.Upsert(context.Background(), Connection{
		UserID: "u", Service: ServiceGoogle, ExternalWorkspaceID: "corp.com",
	}, Token{
		AccessToken:  "stale",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	tok, err := m.Token(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok.AccessToken != "fresh-access" {
		t.Fatalf("expected refreshed access, got %q", tok.AccessToken)
	}
	// Refresh response had empty refresh token — manager must preserve "rt".
	stored, _ := m.store.GetToken(context.Background(), c.ID)
	if stored.RefreshToken != "rt" {
		t.Errorf("refresh token lost on refresh-with-empty-rt: %q", stored.RefreshToken)
	}
	if stored.AccessToken != "fresh-access" {
		t.Errorf("storage not updated: %q", stored.AccessToken)
	}
}

func TestManager_TokenRefreshSingleFlight(t *testing.T) {
	m := newManager(t)
	delete(m.providers, ServiceSlack)
	block := make(chan struct{})
	fake := &fakeRefresher{
		svc: ServiceGoogle,
		respToken: Token{
			AccessToken: "fresh",
			ExpiresAt:   time.Now().Add(time.Hour),
		},
		block: block,
	}
	m.providers[ServiceGoogle] = fake

	c, _ := m.store.Upsert(context.Background(), Connection{
		UserID: "u", Service: ServiceGoogle, ExternalWorkspaceID: "corp.com",
	}, Token{
		AccessToken: "stale", RefreshToken: "rt",
		ExpiresAt: time.Now().Add(-time.Hour),
	})

	// Five concurrent callers — should result in ONE Refresh call.
	results := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := m.Token(context.Background(), c.ID)
			results <- err
		}()
	}
	// Give goroutines a moment to all enter singleflight.
	time.Sleep(50 * time.Millisecond)
	close(block)
	for i := 0; i < 5; i++ {
		if err := <-results; err != nil {
			t.Errorf("call %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&fake.refreshCalls); got != 1 {
		t.Errorf("expected 1 refresh call, got %d", got)
	}
}

func TestManager_TokenRefreshFailureDropsConnection(t *testing.T) {
	m := newManager(t)
	delete(m.providers, ServiceSlack)
	fake := &fakeRefresher{
		svc:     ServiceGoogle,
		respErr: io.EOF, // any error
	}
	m.providers[ServiceGoogle] = fake
	c, _ := m.store.Upsert(context.Background(), Connection{
		UserID: "u", Service: ServiceGoogle, ExternalWorkspaceID: "corp.com",
	}, Token{
		AccessToken: "stale", RefreshToken: "rt",
		ExpiresAt: time.Now().Add(-time.Hour),
	})
	_, err := m.Token(context.Background(), c.ID)
	if err == nil || !strings.Contains(err.Error(), "reconnect") {
		t.Fatalf("expected reconnect-required error, got %v", err)
	}
	// Connection should be gone so the UI re-prompts.
	if _, err := m.store.Get(context.Background(), c.ID); err == nil {
		t.Errorf("expected connection deleted after failed refresh")
	}
}
