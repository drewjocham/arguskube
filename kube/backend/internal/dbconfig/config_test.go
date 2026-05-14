package dbconfig

import (
	"strings"
	"testing"
)

func TestDSN_Postgres(t *testing.T) {
	c := &DBConnection{
		Name: "x", DBType: DBPostgres, Host: "db.internal", Port: 5432,
		User: "argus", Password: "p@ss/word", DBName: "app",
	}
	dsn, err := c.DSN()
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	// Default SSL for postgres should be "require" — never "disable",
	// because we're shipping a one-time setup wizard and silent
	// downgrade-to-cleartext is a footgun.
	if !strings.Contains(dsn, "sslmode=require") {
		t.Fatalf("expected sslmode=require default, got %s", dsn)
	}
	if !strings.Contains(dsn, "argus") || !strings.Contains(dsn, "db.internal:5432") {
		t.Fatalf("dsn missing user/host: %s", dsn)
	}
	// url.UserPassword escapes the slash in the password.
	if strings.Contains(dsn, "p@ss/word") {
		t.Fatalf("password should be URL-escaped in DSN, got %s", dsn)
	}
}

func TestDSN_MySQL(t *testing.T) {
	c := &DBConnection{
		Name: "x", DBType: DBMySQL, Host: "db", Port: 3306,
		User: "u", Password: "p", DBName: "app", SSLMode: SSLRequire,
	}
	dsn, err := c.DSN()
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	if !strings.HasPrefix(dsn, "u:p@tcp(db:3306)/app") {
		t.Fatalf("mysql dsn shape wrong: %s", dsn)
	}
	if !strings.Contains(dsn, "tls=skip-verify") {
		t.Fatalf("mysql SSLRequire should map to tls=skip-verify, got %s", dsn)
	}
}

func TestDSN_SQLite(t *testing.T) {
	c := &DBConnection{Name: "x", DBType: DBSQLite, DBName: "/tmp/foo.db"}
	dsn, err := c.DSN()
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	if dsn != "/tmp/foo.db" {
		t.Fatalf("sqlite dsn should be file path, got %s", dsn)
	}
}

func TestRedact(t *testing.T) {
	c := &DBConnection{Name: "x", DBType: DBPostgres, Host: "h", Port: 5432, Password: "s"}
	r := c.Redact()
	if r.Password != "" {
		t.Fatalf("redact left password: %q", r.Password)
	}
	if c.Password != "s" {
		t.Fatalf("redact mutated original")
	}
}
