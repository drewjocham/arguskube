// Package tlsconfig provides mTLS certificate generation, loading, and
// verification for the agent ↔ SaaS WebSocket tunnel.
//
// Usage flow:
//
//  1. Server (SaaS Hub) calls GenerateCA() once → stores ca.crt + ca.key.
//  2. For each agent registration, server calls IssueAgentCert(ca, agentID)
//     → returns a signed cert + key that the agent stores locally.
//  3. ServerTLSConfig() configures the hub's HTTPS/WSS listener to require
//     client certs signed by the CA.
//  4. AgentTLSConfig() configures the tunnel client to present its cert and
//     verify the server's cert against the CA.
package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

// CACert holds the CA certificate and key used to sign agent certificates.
type CACert struct {
	Cert    *x509.Certificate
	Key     *ecdsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

// AgentCert holds a signed agent certificate and its private key.
type AgentCert struct {
	CertPEM []byte
	KeyPEM  []byte
	AgentID string
}

// GenerateCA creates a new ECDSA P-256 CA certificate for signing agent certs.
// The CA is valid for the specified duration (e.g., 365 * 24 * time.Hour for 1 year).
func GenerateCA(org string, validity time.Duration) (*CACert, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{org},
			CommonName:   "KubeWatcher Agent CA",
		},
		NotBefore:             time.Now().Add(-5 * time.Minute), // Clock skew tolerance.
		NotAfter:              time.Now().Add(validity),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal CA key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &CACert{
		Cert:    cert,
		Key:     key,
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	}, nil
}

// IssueAgentCert creates a client certificate signed by the CA for the given agent.
// The certificate is valid for the specified duration.
func IssueAgentCert(ca *CACert, agentID string, validity time.Duration) (*AgentCert, error) {
	if ca == nil || ca.Cert == nil || ca.Key == nil {
		return nil, fmt.Errorf("ca must be non-nil with valid Cert and Key")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate agent key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"KubeWatcher Agent"},
			CommonName:   agentID,
		},
		NotBefore: time.Now().Add(-5 * time.Minute),
		NotAfter:  time.Now().Add(validity),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("create agent certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal agent key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &AgentCert{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		AgentID: agentID,
	}, nil
}

// ServerTLSConfig returns a tls.Config for the hub that requires client certificates
// signed by the given CA. Only agents with valid certs can connect.
func ServerTLSConfig(caCertPEM, serverCertPEM, serverKeyPEM []byte) (*tls.Config, error) {
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("load server cert: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// AgentTLSConfig returns a tls.Config for the agent tunnel client that presents
// the agent certificate and verifies the server against the CA.
func AgentTLSConfig(caCertPEM, agentCertPEM, agentKeyPEM []byte) (*tls.Config, error) {
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	agentCert, err := tls.X509KeyPair(agentCertPEM, agentKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("load agent cert: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{agentCert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// LoadPEM reads a PEM file from disk.
func LoadPEM(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read PEM %s: %w", path, err)
	}
	return data, nil
}

// WritePEM writes PEM data to a file with restricted permissions.
func WritePEM(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
