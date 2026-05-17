package pkg

import (
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/argues/argus/internal/terminal"
)

// fakeSessionService is a hand-rolled fake. It records calls and lets each
// method return a canned result/error so the handler can be exercised in
// isolation from the real *terminal.SessionManager (which would spawn shells).
type fakeSessionService struct {
	newCalls    atomic.Int64
	closeCalls  atomic.Int64
	listCalls   atomic.Int64
	lastNewArgs struct {
		id, label string
		domain    terminal.Domain
		rows, cols uint16
	}
	lastCloseID string

	newErr   error
	closeErr error
	list     []terminal.SessionInfo
}

func (f *fakeSessionService) NewSession(id string, domain terminal.Domain, label string, rows, cols uint16, _ []string) (*terminal.Session, error) {
	f.newCalls.Add(1)
	f.lastNewArgs.id = id
	f.lastNewArgs.domain = domain
	f.lastNewArgs.label = label
	f.lastNewArgs.rows = rows
	f.lastNewArgs.cols = cols
	if f.newErr != nil {
		return nil, f.newErr
	}
	return &terminal.Session{ID: id, Domain: domain, Label: label}, nil
}

func (f *fakeSessionService) CloseSession(id string) error {
	f.closeCalls.Add(1)
	f.lastCloseID = id
	return f.closeErr
}

func (f *fakeSessionService) ListSessions() []terminal.SessionInfo {
	f.listCalls.Add(1)
	return f.list
}

func newTestHandler(svc SessionService) *TerminalHandler {
	return &TerminalHandler{
		sessions: svc,
		logger:   slog.New(slog.DiscardHandler),
	}
}

func TestStartTerminalSession(t *testing.T) {
	tests := []struct {
		name    string
		newErr  error
		wantErr bool
	}{
		{name: "happy path forwards args", newErr: nil, wantErr: false},
		{name: "propagates manager error", newErr: errors.New("boom"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeSessionService{newErr: tt.newErr}
			h := newTestHandler(svc)

			err := h.StartTerminalSession("sess-1", "k8s", "prod", 24, 80)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if got := svc.newCalls.Load(); got != 1 {
				t.Fatalf("NewSession calls = %d, want 1", got)
			}
			if svc.lastNewArgs.id != "sess-1" ||
				svc.lastNewArgs.domain != terminal.DomainK8s ||
				svc.lastNewArgs.label != "prod" ||
				svc.lastNewArgs.rows != 24 ||
				svc.lastNewArgs.cols != 80 {
				t.Fatalf("forwarded args wrong: %+v", svc.lastNewArgs)
			}
		})
	}
}

func TestCloseTerminalSession(t *testing.T) {
	svc := &fakeSessionService{closeErr: errors.New("not found")}
	h := newTestHandler(svc)

	if err := h.CloseTerminalSession("sess-9"); err == nil {
		t.Fatal("expected error to be propagated")
	}
	if svc.lastCloseID != "sess-9" {
		t.Fatalf("CloseSession got id %q, want %q", svc.lastCloseID, "sess-9")
	}
}

func TestListTerminalSessions(t *testing.T) {
	want := []terminal.SessionInfo{
		{ID: "a", Domain: "k8s", Alive: true},
		{ID: "b", Domain: "kafka", Alive: false},
	}
	svc := &fakeSessionService{list: want}
	h := newTestHandler(svc)

	got := h.ListTerminalSessions()
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%+v, want %+v", i, got[i], want[i])
		}
	}
}
