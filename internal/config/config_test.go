package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigUsesStatelessTiDBBackend(t *testing.T) {
	cfg := Default()

	if cfg.Backend != BackendTiDB {
		t.Fatalf("Backend = %q, want %q", cfg.Backend, BackendTiDB)
	}
	if cfg.StatusAddr == "" {
		t.Fatal("StatusAddr is empty")
	}
	if cfg.CompatVersion != "6.0" {
		t.Fatalf("CompatVersion = %q, want 6.0", cfg.CompatVersion)
	}
}

func TestLoadTOMLConfigMapsTiUPTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timongo.toml")
	if err := os.WriteFile(path, []byte(`
[mongo]
listen_addr = "0.0.0.0:27017"
compat_version = "6.0"

[status]
listen_addr = "0.0.0.0:28017"

[tidb]
host = "10.0.1.20"
port = 4000
database = "_timongo"
user = "timongo"
password = "secret"
max_open_conns = 4096

[runtime]
stateless = true
cursor_ttl = "10m"
session_ttl = "30m"
transaction_timeout = "60s"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ListenAddr != "0.0.0.0:27017" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.StatusAddr != "0.0.0.0:28017" {
		t.Fatalf("StatusAddr = %q", cfg.StatusAddr)
	}
	wantDSN := "timongo:secret@tcp(10.0.1.20:4000)/_timongo?parseTime=true"
	if cfg.TiDBDSN != wantDSN {
		t.Fatalf("TiDBDSN = %q, want %q", cfg.TiDBDSN, wantDSN)
	}
}

func TestLoadTOMLConfigRejectsStatefulRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timongo.toml")
	if err := os.WriteFile(path, []byte(`
[tidb]
host = "127.0.0.1"
port = 4000

[runtime]
stateless = false
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("LoadFile returned nil error")
	}
}
