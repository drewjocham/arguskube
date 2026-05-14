package saasapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// distload RPC tests. Each method gets a happy-path + a representative
// error-path test. The shared do() machinery is covered separately in
// client_test.go (retry/backoff/circuit-breaker); these tests focus on
// "did this method shape the request and parse the response correctly".

func TestStartDistLoad_HappyPath(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody DistLoadSpec
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"runId":"run-abc"}`))
	}))
	t.Cleanup(srv.Close)

	cli := NewClient(srv.URL, "k", testLogger())
	spec := DistLoadSpec{Name: "test", Destination: "test.topic", Count: 100, Workers: 4}
	runID, err := cli.StartDistLoad(context.Background(), spec)
	if err != nil {
		t.Fatalf("StartDistLoad: %v", err)
	}
	if runID != "run-abc" {
		t.Errorf("runID = %q, want run-abc", runID)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v1/loadtest" {
		t.Errorf("path = %q, want /api/v1/loadtest", gotPath)
	}
	if gotBody.Name != "test" || gotBody.Destination != "test.topic" {
		t.Errorf("body wrong: %+v", gotBody)
	}
}

func TestStartDistLoad_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	_, err := cli.StartDistLoad(context.Background(), DistLoadSpec{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestStartDistLoad_InsufficientCredits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	_, err := cli.StartDistLoad(context.Background(), DistLoadSpec{})
	if !errors.Is(err, ErrInsufficientCredits) {
		t.Errorf("err = %v, want ErrInsufficientCredits", err)
	}
}

func TestGetDistLoadStatus_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/loadtest/run-xyz" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"runId":"run-xyz",
			"state":"running",
			"name":"test",
			"startedAt":"2026-05-14T10:00:00Z",
			"workers":[{"region":"us-east","sent":100,"acked":99}]
		}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	st, err := cli.GetDistLoadStatus(context.Background(), "run-xyz")
	if err != nil {
		t.Fatalf("GetDistLoadStatus: %v", err)
	}
	if st.State != "running" {
		t.Errorf("state = %q, want running", st.State)
	}
	if len(st.Workers) != 1 || st.Workers[0].Sent != 100 {
		t.Errorf("workers parsed wrong: %+v", st.Workers)
	}
}

func TestGetDistLoadStatus_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	_, err := cli.GetDistLoadStatus(context.Background(), "run-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCancelDistLoad_HappyPath(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	if err := cli.CancelDistLoad(context.Background(), "run-abc"); err != nil {
		t.Errorf("CancelDistLoad: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
}

func TestGetCreditBalance_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"balance":1234.5}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	b, err := cli.GetCreditBalance(context.Background())
	if err != nil {
		t.Fatalf("GetCreditBalance: %v", err)
	}
	if b != 1234.5 {
		t.Errorf("balance = %v, want 1234.5", b)
	}
}

func TestGetCreditHistory_NilSliceReturnsEmpty(t *testing.T) {
	// The method documents that it returns an empty slice rather
	// than a nil slice when the SaaS side sends no transactions —
	// frontend renders the empty-state correctly only with that
	// guarantee.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"transactions":null}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	txs, err := cli.GetCreditHistory(context.Background())
	if err != nil {
		t.Fatalf("GetCreditHistory: %v", err)
	}
	if txs == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(txs) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(txs))
	}
}

func TestGetCreditHistory_PopulatedRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"transactions":[
			{"id":"tx-1","amount":-50.0,"type":"debit","runId":"run-1","createdAt":"2026-05-01T10:00:00Z","note":"smoke test"},
			{"id":"tx-2","amount":1000.0,"type":"credit","createdAt":"2026-05-01T09:00:00Z"}
		]}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	txs, err := cli.GetCreditHistory(context.Background())
	if err != nil {
		t.Fatalf("GetCreditHistory: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(txs))
	}
	if txs[0].ID != "tx-1" || txs[0].Amount != -50.0 || txs[0].Type != "debit" {
		t.Errorf("tx[0] parsed wrong: %+v", txs[0])
	}
}

func TestGetDistLoadHistory_NilSliceReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"runs":null}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	runs, err := cli.GetDistLoadHistory(context.Background())
	if err != nil {
		t.Fatalf("GetDistLoadHistory: %v", err)
	}
	if runs == nil || len(runs) != 0 {
		t.Errorf("expected empty slice, got %v", runs)
	}
}

func TestEstimateCost_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/loadtest/estimate" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"estimatedCredits":42.5,"breakdown":{"vmMinutes":10.0,"dataTransfer":2.5,"regionPremium":30.0}}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	c, err := cli.EstimateCost(context.Background(), DistLoadSpec{Count: 100})
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if c != 42.5 {
		t.Errorf("credits = %v, want 42.5", c)
	}
}

func TestEstimateCost_InvalidSpec422(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`destination must be set`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	_, err := cli.EstimateCost(context.Background(), DistLoadSpec{})
	if err == nil {
		t.Fatal("expected error")
	}
	// 422 carries the body in the error message so the frontend
	// can surface the precise validation failure.
	if !contains(err.Error(), "destination must be set") {
		t.Errorf("err = %q, expected to include 422 body", err.Error())
	}
}

func TestListRegions_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"regions":[
			{"provider":"aws","region":"us-east-1","label":"US East (N. Virginia)","instanceTypes":["t3.small","t3.medium"],"defaultType":"t3.small"},
			{"provider":"gcp","region":"us-central1","label":"US Central (Iowa)","instanceTypes":["e2-small"],"defaultType":"e2-small"}
		]}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	regions, err := cli.ListRegions(context.Background())
	if err != nil {
		t.Fatalf("ListRegions: %v", err)
	}
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}
	if regions[0].Provider != "aws" || regions[1].DefaultType != "e2-small" {
		t.Errorf("regions parsed wrong: %+v", regions)
	}
}

func TestListRegions_NilReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"regions":null}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	regions, err := cli.ListRegions(context.Background())
	if err != nil {
		t.Fatalf("ListRegions: %v", err)
	}
	if regions == nil {
		t.Error("expected non-nil slice")
	}
	if len(regions) != 0 {
		t.Errorf("expected empty slice, got %d", len(regions))
	}
}

func TestGetUsage_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"totalCreditsUsed":5000.0,"thisMonth":1200.5,"runsThisMonth":7,"avgCostPerRun":171.5}`))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	u, err := cli.GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if u.RunsThisMonth != 7 || u.AvgCostPerRun != 171.5 {
		t.Errorf("usage parsed wrong: %+v", u)
	}
}

func TestNotConfigured_AppliesToEveryMethod(t *testing.T) {
	// An unconfigured client (empty API key) must short-circuit
	// EVERY method with ErrNotConfigured. The frontend's "connect
	// your account" flow depends on this — if any method bypassed
	// the check, the user would see a noisy 401 cascade on first
	// open.
	cli := NewClient("https://saas.example", "", testLogger())

	cases := []struct {
		name string
		fn   func() error
	}{
		{"StartDistLoad", func() error { _, e := cli.StartDistLoad(context.Background(), DistLoadSpec{}); return e }},
		{"GetDistLoadStatus", func() error { _, e := cli.GetDistLoadStatus(context.Background(), "x"); return e }},
		{"CancelDistLoad", func() error { return cli.CancelDistLoad(context.Background(), "x") }},
		{"GetDistLoadResult", func() error { _, e := cli.GetDistLoadResult(context.Background(), "x"); return e }},
		{"GetCreditBalance", func() error { _, e := cli.GetCreditBalance(context.Background()); return e }},
		{"GetCreditHistory", func() error { _, e := cli.GetCreditHistory(context.Background()); return e }},
		{"GetDistLoadHistory", func() error { _, e := cli.GetDistLoadHistory(context.Background()); return e }},
		{"EstimateCost", func() error { _, e := cli.EstimateCost(context.Background(), DistLoadSpec{}); return e }},
		{"ListRegions", func() error { _, e := cli.ListRegions(context.Background()); return e }},
		{"GetUsage", func() error { _, e := cli.GetUsage(context.Background()); return e }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.fn()
			if !errors.Is(err, ErrNotConfigured) {
				t.Errorf("err = %v, want ErrNotConfigured", err)
			}
		})
	}
}

func TestContextCancel_Propagates(t *testing.T) {
	// A canceled outer context must stop the retry loop quickly.
	// We use a server that sleeps longer than the ctx deadline to
	// guarantee a timeout path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cli := NewClient(srv.URL, "k", testLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := cli.GetCreditBalance(ctx)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected ctx-cancel error")
	}
	// Must exit within a few hundred ms — not wait the full
	// retry budget against a slow server.
	if elapsed > 1500*time.Millisecond {
		t.Errorf("ctx cancel took %v, want under 1.5s", elapsed)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
