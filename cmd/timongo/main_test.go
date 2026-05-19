package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RenlySir/timongo/internal/config"
)

func TestBuildStoreRejectsMemoryBackend(t *testing.T) {
	_, _, err := buildStore(config.Config{Backend: "memory"})
	if err == nil {
		t.Fatal("buildStore returned nil error for memory backend")
	}
	if !strings.Contains(err.Error(), "stateless") {
		t.Fatalf("error = %q, want mention of stateless runtime", err.Error())
	}
}

func TestBuildStoreRequiresTiDBDSN(t *testing.T) {
	_, _, err := buildStore(config.Config{Backend: config.BackendTiDB})
	if err == nil {
		t.Fatal("buildStore returned nil error without TiDB DSN")
	}
}

func TestServeConfigFlagLoadsTiUPRenderedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timongo.toml")
	if err := os.WriteFile(path, []byte(`
[mongo]
listen_addr = "127.0.0.1:27017"
compat_version = "6.0"

[status]
listen_addr = "127.0.0.1:28017"

[tidb]
host = "127.0.0.1"
port = 4000
database = "_timongo"

[runtime]
stateless = true
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseServeConfig([]string{"-config", path})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TiDBDSN != "root@tcp(127.0.0.1:4000)/_timongo?parseTime=true" {
		t.Fatalf("TiDBDSN = %q", cfg.TiDBDSN)
	}
}

func TestServeFlagsOverrideConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timongo.toml")
	if err := os.WriteFile(path, []byte(`
[mongo]
listen_addr = "127.0.0.1:27017"

[status]
listen_addr = "127.0.0.1:28017"

[tidb]
host = "127.0.0.1"
port = 4000
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseServeConfig([]string{
		"-config", path,
		"-listen", "0.0.0.0:27018",
		"-tidb-dsn", "root@tcp(10.0.0.1:4000)/app?parseTime=true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "0.0.0.0:27018" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.TiDBDSN != "root@tcp(10.0.0.1:4000)/app?parseTime=true" {
		t.Fatalf("TiDBDSN = %q", cfg.TiDBDSN)
	}
}

func TestTiUPSplitTopologyCommand(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "topology.yaml")
	tidbOutput := filepath.Join(dir, "tidb.yaml")
	timongoOutput := filepath.Join(dir, "timongo.yaml")
	if err := os.WriteFile(input, []byte(`
global:
  user: tidb
pd_servers:
  - host: 10.0.1.30
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
    tidb_host: 10.0.1.20
    tidb_port: 4000
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runTiUPSplitTopology([]string{"-input", input, "-tidb-output", tidbOutput, "-timongo-output", timongoOutput}); err != nil {
		t.Fatal(err)
	}
	tidbRaw, err := os.ReadFile(tidbOutput)
	if err != nil {
		t.Fatal(err)
	}
	timongoRaw, err := os.ReadFile(timongoOutput)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tidbRaw), "timongo_servers") {
		t.Fatalf("tidb output contains timongo section:\n%s", tidbRaw)
	}
	if !strings.Contains(string(timongoRaw), "timongo_servers") {
		t.Fatalf("timongo output missing timongo section:\n%s", timongoRaw)
	}
}

func TestTiUPRenderTimongoCommand(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "topology.yaml")
	output := filepath.Join(dir, "rendered")
	if err := os.WriteFile(input, []byte(`
global:
  user: tidb
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
    deploy_dir: /tidb-deploy/timongo-27017
    log_dir: /tidb-deploy/timongo-27017/log
    tidb_host: 10.0.1.20
    tidb_port: 4000
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runTiUPRenderTimongo([]string{"-input", input, "-output", output}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(output, "10.0.1.10", "timongo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `host = "10.0.1.20"`) {
		t.Fatalf("rendered config missing TiDB host:\n%s", raw)
	}
}
