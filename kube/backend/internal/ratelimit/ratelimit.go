// Package ratelimit provides a per-IP token-bucket rate limiter for
// the HTTP API. The default token rate (100 req/s, burst 200) targets
// the same shape the Wails-style RPC sees from a busy SaaS client; a
// browser-typing-fast workload sits comfortably below it while a
// runaway script or misconfigured client trips it inside seconds.
//
// The limiter is INTENTIONALLY local-state, not Redis-backed. SaaS
// will eventually need a distributed limiter, but landing that now
// would couple this PR to multi-tenancy work. The interface is small
// enough to swap later — see ratelimit.PerIP for the seam.
package ratelimit

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// PerIP is a thread-safe map of client IP → *rate.Limiter. Callers
// build one and reuse it for the lifetime of the server.
type PerIP struct {
	rps   rate.Limit
	burst int

	mu      sync.Mutex
	visitor map[string]*visitor
	ttl     time.Duration
}

type visitor struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// New returns a PerIP limiter with the given steady-state requests-
// per-second and burst budget. ttl controls how long an idle client's
// bucket is kept around before being evicted — set to 0 for the
// default of 10 * (burst seconds), capped to 10 minutes.
func New(rps float64, burst int, ttl time.Duration) *PerIP {
	if ttl <= 0 {
		ttl = time.Duration(float64(burst)*10) * time.Second
		if ttl > 10*time.Minute {
			ttl = 10 * time.Minute
		}
	}
	return &PerIP{
		rps:     rate.Limit(rps),
		burst:   burst,
		visitor: make(map[string]*visitor),
		ttl:     ttl,
	}
}

// Allow reports whether the given client IP may make one more request
// right now. It's race-free against concurrent calls from any number
// of goroutines.
func (p *PerIP) Allow(ip string) bool {
	p.mu.Lock()
	v, ok := p.visitor[ip]
	if !ok {
		v = &visitor{lim: rate.NewLimiter(p.rps, p.burst)}
		p.visitor[ip] = v
	}
	v.lastSeen = time.Now()
	p.mu.Unlock()
	return v.lim.Allow()
}

// Reserve returns the *rate.Reservation for callers that want to
// block rather than reject. Most HTTP middleware should use Allow.
func (p *PerIP) Reserve(ip string) *rate.Reservation {
	p.mu.Lock()
	v, ok := p.visitor[ip]
	if !ok {
		v = &visitor{lim: rate.NewLimiter(p.rps, p.burst)}
		p.visitor[ip] = v
	}
	v.lastSeen = time.Now()
	p.mu.Unlock()
	return v.lim.Reserve()
}

// Cleanup evicts visitors that haven't been seen within the TTL.
// Intended to run on a ticker; one call walks the entire map under
// the lock, so callers should not invoke it more often than once a
// minute under realistic load. Returns the number of entries
// evicted, useful for metrics or tests.
func (p *PerIP) Cleanup() int {
	cutoff := time.Now().Add(-p.ttl)
	p.mu.Lock()
	defer p.mu.Unlock()
	evicted := 0
	for ip, v := range p.visitor {
		if v.lastSeen.Before(cutoff) {
			delete(p.visitor, ip)
			evicted++
		}
	}
	return evicted
}

// Size returns the current number of tracked visitors. Helpful in tests.
func (p *PerIP) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.visitor)
}

// Middleware wraps the next handler with per-IP rate limiting.
// Requests over the configured rate return 429 Too Many Requests with
// a Retry-After hint computed from the limiter's reservation.
//
// The client IP is taken from the first hop in X-Forwarded-For when
// present (SaaS behind a load balancer), otherwise r.RemoteAddr.
// Loopback callers (127.0.0.1, ::1) bypass the limit — the desktop
// app's own Wails frontend hits this path in a tight loop on first
// paint, and there's no DoS risk from the same machine the binary
// already runs on.
func Middleware(p *PerIP) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if isLoopback(ip) || p.Allow(ip) {
				next.ServeHTTP(w, r)
				return
			}
			// Hint the client when to retry. Use the limiter's
			// reservation to compute it accurately rather than guessing.
			res := p.Reserve(ip)
			// We're not actually going to use the reservation — cancel
			// it so the slot we just consumed goes back to the pool.
			delay := res.Delay()
			res.Cancel()
			retrySec := int(delay.Seconds())
			if retrySec < 1 {
				retrySec = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retrySec))
			http.Error(w, "rate limited", http.StatusTooManyRequests)
		})
	}
}

func clientIP(r *http.Request) string {
	// Trust X-Forwarded-For only for the FIRST hop. Any client can set
	// arbitrary values, but our SaaS LB is the only legitimate sender
	// and it places the real client IP first.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := indexByte(xff, ','); comma >= 0 {
			return trimSpace(xff[:comma])
		}
		return trimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isLoopback(ip string) bool {
	if ip == "" {
		return false
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

// Tiny inline string helpers — avoid importing "strings" for two ops.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
