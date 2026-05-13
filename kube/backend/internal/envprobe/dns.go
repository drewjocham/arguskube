package envprobe

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"
)

// Resolver is the slice of net.Resolver we depend on, exposed as an
// interface so tests can inject a fake without poisoning system DNS.
type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// APIHostProvider returns the API server URL the probe should hit. It's a
// function (not a string) so a context switch is observed automatically
// without re-registering the probe.
type APIHostProvider func() string

// DNSProbe resolves the API server hostname. A failure here is the
// single most common "Argus doesn't work" signal — VPN off, split-
// horizon DNS, or a cluster name pointing at a private network. The
// remediation we surface depends on what we can detect; for now we
// emit a stable ActionID the frontend dispatches.
type DNSProbe struct {
	host     APIHostProvider
	resolver Resolver
}

func NewDNSProbe(host APIHostProvider, resolver Resolver) *DNSProbe {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &DNSProbe{host: host, resolver: resolver}
}

func (p *DNSProbe) ID() string { return "envprobe.dns" }

func (p *DNSProbe) Run(ctx context.Context) Result {
	apiURL := p.host()
	res := Result{ID: p.ID(), Title: "Cluster hostname resolves"}
	if apiURL == "" {
		res.Status = Warn
		res.Detail = "No cluster selected yet."
		return res
	}

	host, err := hostnameOf(apiURL)
	if err != nil {
		res.Status = Warn
		res.Detail = "Could not parse API server URL: " + err.Error()
		return res
	}
	// Skip raw IPs — DNS isn't in play, and resolution would succeed
	// trivially without telling us anything useful.
	if net.ParseIP(host) != nil {
		res.Status = OK
		res.Detail = fmt.Sprintf("API server addressed by IP (%s).", host)
		return res
	}

	start := time.Now()
	addrs, err := p.resolver.LookupHost(ctx, host)
	res.Latency = time.Since(start)
	if err != nil {
		res.Status = Todo
		res.Detail = fmt.Sprintf("%s does not resolve. Likely cause: VPN off or split-horizon DNS.", host)
		res.ActionLabel = "Re-check"
		res.ActionID = "envprobe.recheck"
		return res
	}
	if len(addrs) == 0 {
		res.Status = Todo
		res.Detail = fmt.Sprintf("%s resolves to zero addresses.", host)
		res.ActionLabel = "Re-check"
		res.ActionID = "envprobe.recheck"
		return res
	}
	res.Status = OK
	res.Detail = fmt.Sprintf("%s → %s", host, addrs[0])
	return res
}

func hostnameOf(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Host != "" {
		return u.Hostname(), nil
	}
	// Some kubeconfigs store host:port without a scheme; treat as host.
	return rawURL, nil
}
