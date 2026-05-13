package envprobe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"testing"
	"time"
)

// --- DNS probe -------------------------------------------------------

type stubResolver struct {
	addrs []string
	err   error
}

func (s stubResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return s.addrs, s.err
}

func TestDNSProbe_NoCluster(t *testing.T) {
	p := NewDNSProbe(func() string { return "" }, stubResolver{})
	res := p.Run(context.Background())
	if res.Status != Warn {
		t.Errorf("empty host should be Warn, got %s", res.Status)
	}
}

func TestDNSProbe_Resolves(t *testing.T) {
	p := NewDNSProbe(
		func() string { return "https://prod.example:6443" },
		stubResolver{addrs: []string{"10.0.0.1"}},
	)
	res := p.Run(context.Background())
	if res.Status != OK {
		t.Errorf("want OK, got %s — detail=%s", res.Status, res.Detail)
	}
	if res.Detail == "" {
		t.Errorf("detail should mention the resolved address")
	}
}

func TestDNSProbe_DoesNotResolve(t *testing.T) {
	p := NewDNSProbe(
		func() string { return "https://prod.example:6443" },
		stubResolver{err: fmt.Errorf("no such host")},
	)
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Errorf("unresolved host should be Todo, got %s", res.Status)
	}
	if res.ActionLabel == "" {
		t.Errorf("Todo row should expose a re-check action")
	}
}

func TestDNSProbe_ZeroAddrsTreatedAsFailure(t *testing.T) {
	p := NewDNSProbe(
		func() string { return "https://prod.example:6443" },
		stubResolver{addrs: nil},
	)
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Errorf("zero-addr resolve should be Todo, got %s", res.Status)
	}
}

func TestDNSProbe_RawIPSkipsResolution(t *testing.T) {
	// If the resolver were called it would error — proves we short-circuited.
	p := NewDNSProbe(
		func() string { return "https://10.0.0.1:6443" },
		stubResolver{err: fmt.Errorf("should not be called")},
	)
	res := p.Run(context.Background())
	if res.Status != OK {
		t.Errorf("IP host should bypass DNS, got %s", res.Status)
	}
}

// --- Clock skew probe -----------------------------------------------

type stubClock struct{ t time.Time }

func (s *stubClock) Now() time.Time { return s.t }
func (s *stubClock) advance(d time.Duration) { s.t = s.t.Add(d) }

type stubFetcher struct {
	t   time.Time
	err error
}

func (s stubFetcher) ServerTime(ctx context.Context, apiURL string) (time.Time, error) {
	return s.t, s.err
}

func TestClockSkewProbe_InSync(t *testing.T) {
	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	p := NewClockSkewProbe(
		func() string { return "https://prod:6443" },
		stubFetcher{t: now},
		30*time.Second,
	)
	p.clock = &stubClock{t: now}
	res := p.Run(context.Background())
	if res.Status != OK {
		t.Errorf("0 skew should be OK, got %s", res.Status)
	}
}

func TestClockSkewProbe_OverThreshold(t *testing.T) {
	serverTime := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	localTime := serverTime.Add(45 * time.Second)
	p := NewClockSkewProbe(
		func() string { return "https://prod:6443" },
		stubFetcher{t: serverTime},
		30*time.Second,
	)
	p.clock = &stubClock{t: localTime}
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Errorf("45s skew should be Todo, got %s", res.Status)
	}
	if res.ActionLabel == "" {
		t.Errorf("Todo skew row should expose an action")
	}
}

func TestClockSkewProbe_FetcherErrorIsSoftWarn(t *testing.T) {
	// Reachability errors are owned by other probes — don't double-report.
	p := NewClockSkewProbe(
		func() string { return "https://prod:6443" },
		stubFetcher{err: errors.New("dial tcp: i/o timeout")},
		30*time.Second,
	)
	p.clock = &stubClock{t: time.Now()}
	res := p.Run(context.Background())
	if res.Status != Warn {
		t.Errorf("fetcher error should be Warn, got %s", res.Status)
	}
}

func TestClockSkewProbe_NoHost(t *testing.T) {
	p := NewClockSkewProbe(func() string { return "" }, stubFetcher{}, 30*time.Second)
	if got := p.Run(context.Background()).Status; got != Warn {
		t.Errorf("empty host should be Warn, got %s", got)
	}
}

// --- TLS chain probe ------------------------------------------------

type stubDialer struct {
	err error
}

func (s stubDialer) Dial(ctx context.Context, address string, cfg *tls.Config) error {
	return s.err
}

func TestTLSChainProbe_Valid(t *testing.T) {
	p := NewTLSChainProbe(
		func() string { return "https://prod:6443" },
		stubDialer{err: nil},
	)
	res := p.Run(context.Background())
	if res.Status != OK {
		t.Errorf("clean handshake should be OK, got %s", res.Status)
	}
}

func TestTLSChainProbe_UnknownAuthorityIsCorpCATodo(t *testing.T) {
	p := NewTLSChainProbe(
		func() string { return "https://prod:6443" },
		stubDialer{err: x509.UnknownAuthorityError{}},
	)
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Errorf("unknown CA should be Todo, got %s", res.Status)
	}
	if res.ActionID != "envprobe.trust-corp-ca" {
		t.Errorf("expected the trust-corp-ca action id, got %s", res.ActionID)
	}
}

func TestTLSChainProbe_HostnameMismatch(t *testing.T) {
	p := NewTLSChainProbe(
		func() string { return "https://prod:6443" },
		stubDialer{err: x509.HostnameError{Host: "prod"}},
	)
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Errorf("hostname mismatch should be Todo, got %s", res.Status)
	}
}

func TestTLSChainProbe_GenericFailureIsWarn(t *testing.T) {
	p := NewTLSChainProbe(
		func() string { return "https://prod:6443" },
		stubDialer{err: errors.New("dial tcp: i/o timeout")},
	)
	res := p.Run(context.Background())
	if res.Status != Warn {
		t.Errorf("generic TLS failure should be Warn (reachability owned elsewhere), got %s", res.Status)
	}
}

func TestTLSChainProbe_PlainHTTPSkipsHandshake(t *testing.T) {
	p := NewTLSChainProbe(
		func() string { return "http://prod:6443" },
		stubDialer{err: errors.New("should not be called")},
	)
	res := p.Run(context.Background())
	if res.Status != OK {
		t.Errorf("plaintext API should bypass chain, got %s", res.Status)
	}
}

func TestTLSChainProbe_AddsDefaultPort(t *testing.T) {
	var capturedAddr string
	p := NewTLSChainProbe(
		func() string { return "https://prod" },
		dialerFunc(func(ctx context.Context, address string, cfg *tls.Config) error {
			capturedAddr = address
			return nil
		}),
	)
	p.Run(context.Background())
	if capturedAddr != "prod:443" {
		t.Errorf("default port should be 443, got %s", capturedAddr)
	}
}

type dialerFunc func(ctx context.Context, address string, cfg *tls.Config) error

func (f dialerFunc) Dial(ctx context.Context, address string, cfg *tls.Config) error {
	return f(ctx, address, cfg)
}
