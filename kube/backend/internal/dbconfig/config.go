// Package dbconfig manages user-registered database connections for
// DBAgent: the list of databases Argus is allowed to introspect, plus
// their (encrypted) credentials. Each DBConnection is the rendezvous
// point between the UI (settings list), the MCP db_analyze_* tools, and
// the per-DB Python agent.
//
// Persistence lives in the shared sqlitedb (table db_connections);
// passwords are AES-256-GCM encrypted with a master key kept in
// secretstore (macOS Keychain on darwin, in-memory elsewhere — same
// trust model as the session token).
package dbconfig

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// DBType is the canonical name of a supported database engine. The MCP
// tool name (db_analyze_<type>) and the Python agent module
// (db_agents/<type>_agent.py) are derived from this string, so changing
// it is a breaking change.
type DBType string

const (
	DBPostgres   DBType = "postgres"
	DBMySQL      DBType = "mysql"
	DBOracle     DBType = "oracle"
	DBMSSQL      DBType = "mssql"
	DBSQLite     DBType = "sqlite"
	DBClickHouse DBType = "clickhouse"
)

// supported lists the Phase 1 SQL engines. Phase 2 (mongo/redis/etc.)
// will add entries here; the store doesn't care about the value, only
// Validate() does.
var supported = map[DBType]bool{
	DBPostgres: true, DBMySQL: true, DBOracle: true,
	DBMSSQL: true, DBSQLite: true, DBClickHouse: true,
}

// SSLMode mirrors libpq's sslmode names; non-postgres dialects map
// "disable"/"require"/"verify-full" onto their own TLS knobs in DSN().
type SSLMode string

const (
	SSLDisable    SSLMode = "disable"
	SSLRequire    SSLMode = "require"
	SSLVerifyCA   SSLMode = "verify-ca"
	SSLVerifyFull SSLMode = "verify-full"
)

// DBConnection is one row in db_connections. Password is the plaintext
// form used in memory; the store encrypts it on write and decrypts on
// read, so callers never deal with ciphertext directly.
type DBConnection struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	DBType    DBType   `json:"db_type"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	User      string   `json:"user"`
	Password  string   `json:"password,omitempty"`
	DBName    string   `json:"db_name"`
	SSLMode   SSLMode  `json:"ssl_mode"`
	PoolSize  int      `json:"pool_size"`
	Tags      []string `json:"tags"`
	Enabled   bool     `json:"enabled"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// Validate enforces the invariants the rest of the system relies on:
// every DBConnection that reaches a Tester or an MCP tool has a known
// type, a non-empty name, and (for network DBs) a host. SQLite is the
// only exception — its "host" is a file path on DBName.
func (c *DBConnection) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("dbconfig: name is required")
	}
	if !supported[c.DBType] {
		return fmt.Errorf("dbconfig: unsupported db_type %q", c.DBType)
	}
	if c.DBType == DBSQLite {
		if strings.TrimSpace(c.DBName) == "" {
			return fmt.Errorf("dbconfig: sqlite requires db_name (file path)")
		}
		return nil
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("dbconfig: host is required for %s", c.DBType)
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("dbconfig: port %d out of range", c.Port)
	}
	return nil
}

// DSN renders a driver-compatible connection string. Each dialect has
// its own quirks; we centralize them here so the MCP tools and the
// Tester share one source of truth.
func (c *DBConnection) DSN() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	switch c.DBType {
	case DBPostgres:
		// postgres://user:pass@host:port/dbname?sslmode=...
		u := &url.URL{
			Scheme: "postgres",
			Host:   c.Host + ":" + strconv.Itoa(c.Port),
			Path:   "/" + c.DBName,
		}
		if c.User != "" {
			if c.Password != "" {
				u.User = url.UserPassword(c.User, c.Password)
			} else {
				u.User = url.User(c.User)
			}
		}
		q := u.Query()
		q.Set("sslmode", string(sslOrDefault(c.SSLMode, SSLRequire)))
		u.RawQuery = q.Encode()
		return u.String(), nil

	case DBMySQL:
		// go-sql-driver/mysql: user:pass@tcp(host:port)/dbname?tls=...
		var b strings.Builder
		if c.User != "" {
			b.WriteString(c.User)
			if c.Password != "" {
				b.WriteByte(':')
				b.WriteString(c.Password)
			}
			b.WriteByte('@')
		}
		fmt.Fprintf(&b, "tcp(%s:%d)/%s", c.Host, c.Port, c.DBName)
		if tls := mysqlTLS(c.SSLMode); tls != "" {
			b.WriteString("?tls=")
			b.WriteString(tls)
		}
		return b.String(), nil

	case DBMSSQL:
		// sqlserver://user:pass@host:port?database=...&encrypt=...
		u := &url.URL{
			Scheme: "sqlserver",
			Host:   c.Host + ":" + strconv.Itoa(c.Port),
		}
		if c.User != "" {
			if c.Password != "" {
				u.User = url.UserPassword(c.User, c.Password)
			} else {
				u.User = url.User(c.User)
			}
		}
		q := u.Query()
		if c.DBName != "" {
			q.Set("database", c.DBName)
		}
		q.Set("encrypt", mssqlEncrypt(c.SSLMode))
		u.RawQuery = q.Encode()
		return u.String(), nil

	case DBOracle:
		// godror: user="u" password="p" connectString="host:port/service"
		var b strings.Builder
		fmt.Fprintf(&b, `user="%s" password="%s" connectString="%s:%d/%s"`,
			c.User, c.Password, c.Host, c.Port, c.DBName)
		return b.String(), nil

	case DBSQLite:
		// modernc.org/sqlite: file path with optional pragmas. We don't
		// add WAL here because user-attached databases might be
		// read-only; the MCP tool can opt in via query params.
		return c.DBName, nil

	case DBClickHouse:
		// clickhouse-go/v2: clickhouse://user:pass@host:port/dbname?secure=...
		u := &url.URL{
			Scheme: "clickhouse",
			Host:   c.Host + ":" + strconv.Itoa(c.Port),
			Path:   "/" + c.DBName,
		}
		if c.User != "" {
			if c.Password != "" {
				u.User = url.UserPassword(c.User, c.Password)
			} else {
				u.User = url.User(c.User)
			}
		}
		q := u.Query()
		if c.SSLMode != "" && c.SSLMode != SSLDisable {
			q.Set("secure", "true")
		}
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	return "", fmt.Errorf("dbconfig: DSN not implemented for %s", c.DBType)
}

func sslOrDefault(m, d SSLMode) SSLMode {
	if m == "" {
		return d
	}
	return m
}

func mysqlTLS(m SSLMode) string {
	switch m {
	case SSLDisable, "":
		return ""
	case SSLRequire:
		return "skip-verify"
	default:
		return "true"
	}
}

func mssqlEncrypt(m SSLMode) string {
	switch m {
	case SSLDisable:
		return "disable"
	case SSLRequire, SSLVerifyCA:
		return "true"
	default:
		return "true"
	}
}

// Redact returns a copy with Password stripped, safe for logging and
// for JSON responses going back to the UI (the UI uses a separate
// "set password" flow rather than echoing it).
func (c *DBConnection) Redact() DBConnection {
	out := *c
	out.Password = ""
	return out
}
