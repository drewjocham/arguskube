package tlsconfig_test

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/argues/kube-watcher/internal/tlsconfig"
)

func TestGenerateCA(t *testing.T) {
	ca, err := tlsconfig.GenerateCA("TestOrg", 365*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA() failed: %v", err)
	}
	if ca == nil {
		t.Fatal("GenerateCA() returned nil")
	}
	if len(ca.CertPEM) == 0 {
		t.Error("CertPEM is empty")
	}
	if len(ca.KeyPEM) == 0 {
		t.Error("KeyPEM is empty")
	}
	if ca.Cert == nil {
		t.Error("Cert is nil")
	}
	if ca.Key == nil {
		t.Error("Key is nil")
	}
	if !ca.Cert.IsCA {
		t.Error("generated cert should be a CA")
	}
}

func TestIssueAgentCert(t *testing.T) {
	ca, err := tlsconfig.GenerateCA("TestOrg", 365*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA() failed: %v", err)
	}

	agentID := "agent-test-001"
	agentCert, err := tlsconfig.IssueAgentCert(ca, agentID, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("IssueAgentCert() failed: %v", err)
	}
	if agentCert == nil {
		t.Fatal("IssueAgentCert() returned nil")
	}
	if agentCert.AgentID != agentID {
		t.Errorf("AgentID = %q, want %q", agentCert.AgentID, agentID)
	}
	if len(agentCert.CertPEM) == 0 {
		t.Error("CertPEM is empty")
	}
	if len(agentCert.KeyPEM) == 0 {
		t.Error("KeyPEM is empty")
	}
}

func TestAgentTLSConfig(t *testing.T) {
	ca, err := tlsconfig.GenerateCA("TestOrg", 365*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA() failed: %v", err)
	}

	agentCert, err := tlsconfig.IssueAgentCert(ca, "agent-001", 90*24*time.Hour)
	if err != nil {
		t.Fatalf("IssueAgentCert() failed: %v", err)
	}

	tlsCfg, err := tlsconfig.AgentTLSConfig(ca.CertPEM, agentCert.CertPEM, agentCert.KeyPEM)
	if err != nil {
		t.Fatalf("AgentTLSConfig() failed: %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("AgentTLSConfig() returned nil")
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(tlsCfg.Certificates))
	}
	if tlsCfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = 0x%04x, want 0x%04x (TLS 1.3)", tlsCfg.MinVersion, tls.VersionTLS13)
	}
}

func TestServerTLSConfig(t *testing.T) {
	ca, err := tlsconfig.GenerateCA("TestOrg", 365*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA() failed: %v", err)
	}

	// For server TLS, we need a server cert signed by the CA
	serverCert, err := tlsconfig.IssueAgentCert(ca, "server", 90*24*time.Hour)
	if err != nil {
		t.Fatalf("IssueAgentCert() for server failed: %v", err)
	}

	tlsCfg, err := tlsconfig.ServerTLSConfig(ca.CertPEM, serverCert.CertPEM, serverCert.KeyPEM)
	if err != nil {
		t.Fatalf("ServerTLSConfig() failed: %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("ServerTLSConfig() returned nil")
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(tlsCfg.Certificates))
	}
	if tlsCfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Error("ClientAuth should be RequireAndVerifyClientCert")
	}
	if tlsCfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = 0x%04x, want 0x%04x (TLS 1.3)", tlsCfg.MinVersion, tls.VersionTLS13)
	}
}

func TestCAInvalidShortValidity(t *testing.T) {
	// Very short validity should still work (the cert is valid from -5min onward)
	ca, err := tlsconfig.GenerateCA("TestOrg", 1*time.Second)
	if err != nil {
		t.Fatalf("GenerateCA() with 1s validity failed: %v", err)
	}
	if ca == nil {
		t.Fatal("GenerateCA() returned nil")
	}
}

func TestAgentCertWithShortValidity(t *testing.T) {
	ca, err := tlsconfig.GenerateCA("TestOrg", 365*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA() failed: %v", err)
	}

	// Very short validity should also work
	agentCert, err := tlsconfig.IssueAgentCert(ca, "agent-short", 1*time.Second)
	if err != nil {
		t.Fatalf("IssueAgentCert() with 1s validity failed: %v", err)
	}
	if agentCert == nil {
		t.Fatal("IssueAgentCert() returned nil")
	}
}

func TestAgentTLSConfigInvalidCA(t *testing.T) {
	_, err := tlsconfig.AgentTLSConfig([]byte("invalid-pem"), []byte("invalid-pem"), []byte("invalid-pem"))
	if err == nil {
		t.Fatal("AgentTLSConfig() with invalid PEM should fail")
	}
}

func TestServerTLSConfigInvalidCA(t *testing.T) {
	_, err := tlsconfig.ServerTLSConfig([]byte("invalid-pem"), []byte("invalid-pem"), []byte("invalid-pem"))
	if err == nil {
		t.Fatal("ServerTLSConfig() with invalid PEM should fail")
	}
}

func TestIssueAgentCertNilCA(t *testing.T) {
	_, err := tlsconfig.IssueAgentCert(nil, "agent-001", 90*24*time.Hour)
	if err == nil {
		t.Fatal("IssueAgentCert() with nil CA should fail")
	}
}

func TestLoadWritePEMRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.pem"
	data := []byte("-----BEGIN CERTIFICATE-----\nZmFrZQ==\n-----END CERTIFICATE-----\n")

	if err := tlsconfig.WritePEM(path, data); err != nil {
		t.Fatalf("WritePEM() failed: %v", err)
	}

	read, err := tlsconfig.LoadPEM(path)
	if err != nil {
		t.Fatalf("LoadPEM() failed: %v", err)
	}
	if string(read) != string(data) {
		t.Errorf("LoadPEM returned different data")
	}
}

func TestLoadPEMNotFound(t *testing.T) {
	_, err := tlsconfig.LoadPEM("/tmp/nonexistent-file-xyz.pem")
	if err == nil {
		t.Fatal("LoadPEM() with nonexistent path should fail")
	}
}
