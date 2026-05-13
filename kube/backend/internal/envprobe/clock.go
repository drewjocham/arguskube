package envprobe

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// DateFetcher is the seam we use to read the API server's Date header.
// In production it does a single HEAD request; in tests a stub returns a
// canned time without actually opening a socket.
type DateFetcher interface {
	ServerTime(ctx context.Context, apiURL string) (time.Time, error)
}

// Clock is the local wall clock, behind an interface so tests can drift
// it without monkey-patching time.Now.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// SystemClock is the production wall-clock used by ClockSkewProbe by
// default. Exposed so other probes can share it without duplicating.
var SystemClock Clock = realClock{}

// ClockSkewProbe compares the local clock to the API server's reported
// time. A skew larger than the threshold causes intermittent JWT and TLS
// failures that are notoriously hard to diagnose from the user side, so
// catching it here is high-value.
type ClockSkewProbe struct {
	host      APIHostProvider
	fetcher   DateFetcher
	clock     Clock
	threshold time.Duration
}

func NewClockSkewProbe(host APIHostProvider, fetcher DateFetcher, threshold time.Duration) *ClockSkewProbe {
	if fetcher == nil {
		fetcher = defaultDateFetcher{}
	}
	if threshold <= 0 {
		threshold = 30 * time.Second
	}
	return &ClockSkewProbe{
		host:      host,
		fetcher:   fetcher,
		clock:     SystemClock,
		threshold: threshold,
	}
}

func (p *ClockSkewProbe) ID() string { return "envprobe.clock" }

func (p *ClockSkewProbe) Run(ctx context.Context) Result {
	apiURL := p.host()
	res := Result{ID: p.ID(), Title: "Clock matches the cluster"}
	if apiURL == "" {
		res.Status = Warn
		res.Detail = "No cluster selected yet."
		return res
	}

	start := p.clock.Now()
	serverTime, err := p.fetcher.ServerTime(ctx, apiURL)
	// Use the injected clock for elapsed-time so tests with a stubbed
	// clock observe deterministic latency (and zero apparent skew).
	res.Latency = p.clock.Now().Sub(start)
	if err != nil {
		// We don't fail loudly here — DNS / TLS / proxy probes will own
		// reachability errors. Treat this as a soft warning instead of a
		// blocker so the user doesn't see the same root cause three times.
		res.Status = Warn
		res.Detail = "Could not read API server time."
		return res
	}

	// Use the wall-clock midpoint of the request to discount round-trip
	// time. Without this, a slow link would look like a fake skew.
	localMid := p.clock.Now().Add(-res.Latency / 2)
	skew := localMid.Sub(serverTime)
	if skew < 0 {
		skew = -skew
	}
	if skew >= p.threshold {
		res.Status = Todo
		res.Detail = fmt.Sprintf(
			"Clock drift %s exceeds %s — JWT and TLS verification will fail intermittently.",
			skew.Round(time.Second), p.threshold,
		)
		res.ActionLabel = "Open Date & Time"
		res.ActionID = "envprobe.open-datetime"
		return res
	}
	res.Status = OK
	res.Detail = fmt.Sprintf("In sync within %s.", skew.Round(time.Second))
	return res
}

// defaultDateFetcher does a HEAD against the API server's `/version`
// endpoint and reads the `Date` response header. We pick `/version`
// because it's unauthenticated, lightweight, and present on every
// kube-apiserver implementation.
type defaultDateFetcher struct{}

func (defaultDateFetcher) ServerTime(ctx context.Context, apiURL string) (time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, apiURL+"/version", nil)
	if err != nil {
		return time.Time{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	date := resp.Header.Get("Date")
	if date == "" {
		return time.Time{}, fmt.Errorf("missing Date header")
	}
	return time.Parse(http.TimeFormat, date)
}
