package envprobe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"
)

// TLSDialer abstracts the TLS handshake so tests can simulate cert
// failures and corp-proxy intercepts without actually opening a socket.
type TLSDialer interface {
	Dial(ctx context.Context, address string, cfg *tls.Config) error
}

type realTLSDialer struct{}

func (realTLSDialer) Dial(ctx context.Context, address string, cfg *tls.Config) error {
	d := &net.Dialer{Timeout: 4 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", address, cfg)
	if err != nil {
		return err
	}
	return conn.Close()
}

// TLSChainProbe verifies that the API server's TLS certificate chain
// validates against the system trust store. The single most common
// failure here is a corporate MITM proxy — a probe-level OK confirms
// the user is on a clean egress path; a Todo means the user needs to
// trust their corporate CA (one-click action wired by the frontend).
type TLSChainProbe struct {
	host   APIHostProvider
	dialer TLSDialer
}

func NewTLSChainProbe(host APIHostProvider, dialer TLSDialer) *TLSChainProbe {
	if dialer == nil {
		dialer = realTLSDialer{}
	}
	return &TLSChainProbe{host: host, dialer: dialer}
}

func (p *TLSChainProbe) ID() string { return "envprobe.tls" }

func (p *TLSChainProbe) Run(ctx context.Context) Result {
	apiURL := p.host()
	res := Result{ID: p.ID(), Title: "TLS chain to API server"}
	if apiURL == "" {
		res.Status = Warn
		res.Detail = "No cluster selected yet."
		return res
	}

	u, err := url.Parse(apiURL)
	if err != nil || u.Host == "" {
		res.Status = Warn
		res.Detail = "Could not parse API server URL."
		return res
	}
	// kubeconfig API servers almost always run TLS; skip plain HTTP.
	if u.Scheme == "http" {
		res.Status = OK
		res.Detail = "API server is plaintext HTTP — no chain to verify."
		return res
	}

	address := u.Host
	if u.Port() == "" {
		address += ":443"
	}

	start := time.Now()
	err = p.dialer.Dial(ctx, address, &tls.Config{
		ServerName: u.Hostname(),
		MinVersion: tls.VersionTLS12,
	})
	res.Latency = time.Since(start)
	if err == nil {
		res.Status = OK
		res.Detail = fmt.Sprintf("Chain valid (%s).", address)
		return res
	}

	// We distinguish "untrusted CA" (probably a corp proxy) from "other
	// TLS error" because the remediation differs: a CA we can trust;
	// a generic handshake failure usually means VPN/firewall.
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameMismatch x509.HostnameError
	switch {
	case errors.As(err, &unknownAuthority):
		res.Status = Todo
		res.Detail = "API server certificate signed by an untrusted CA — likely a corporate proxy."
		res.ActionLabel = "Trust corporate CA"
		res.ActionID = "envprobe.trust-corp-ca"
	case errors.As(err, &hostnameMismatch):
		res.Status = Todo
		res.Detail = fmt.Sprintf("Certificate hostname mismatch for %s.", u.Hostname())
		res.ActionLabel = "Re-check"
		res.ActionID = "envprobe.recheck"
	default:
		res.Status = Warn
		res.Detail = "TLS handshake failed: " + err.Error()
		res.ActionLabel = "Re-check"
		res.ActionID = "envprobe.recheck"
	}
	return res
}
