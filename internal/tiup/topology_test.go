package tiup

import (
	"strings"
	"testing"
)

func TestParseTopologyRequiresTimongoServerBinding(t *testing.T) {
	raw := []byte(`
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
`)
	_, err := ParseTopology(raw)
	if err == nil {
		t.Fatal("expected missing tidb binding error")
	}
}

func TestParseTopologyAcceptsBoundTimongoServer(t *testing.T) {
	raw := []byte(`
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
    tidb_host: 10.0.1.20
    tidb_port: 4000
    config:
      compat_version: "6.0"
      stateless: true
`)
	topo, err := ParseTopology(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.TimongoServers) != 1 {
		t.Fatalf("timongo servers = %d", len(topo.TimongoServers))
	}
	if topo.TimongoServers[0].TiDBHost != "10.0.1.20" {
		t.Fatalf("TiDBHost = %q", topo.TimongoServers[0].TiDBHost)
	}
}

func TestSplitTopologyRemovesTimongoSectionsForTiUPCluster(t *testing.T) {
	raw := []byte(`
global:
  user: tidb
pd_servers:
  - host: 10.0.1.30
tidb_servers:
  - host: 10.0.1.20
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
    tidb_host: 10.0.1.20
    tidb_port: 4000
timongo_lb_servers:
  - host: 10.0.1.5
    port: 27017
`)

	tidbRaw, timongoRaw, err := SplitTopology(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tidbRaw), "timongo_servers") {
		t.Fatalf("TiDB topology still contains timongo section:\n%s", tidbRaw)
	}
	if !strings.Contains(string(tidbRaw), "pd_servers") {
		t.Fatalf("TiDB topology missing pd_servers:\n%s", tidbRaw)
	}
	if !strings.Contains(string(timongoRaw), "timongo_servers") {
		t.Fatalf("timongo topology missing timongo_servers:\n%s", timongoRaw)
	}
	if !strings.Contains(string(timongoRaw), "global:") {
		t.Fatalf("timongo topology missing global:\n%s", timongoRaw)
	}
}

func TestRenderTimongoFilesUsesBoundTiDBServer(t *testing.T) {
	raw := []byte(`
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
    config:
      compat_version: "6.0"
      stateless: true
      cursor_ttl: "10m"
      session_ttl: "30m"
      transaction_timeout: "60s"
      tidb_max_open_conns: 4096
`)
	topo, err := ParseTopology(raw)
	if err != nil {
		t.Fatal(err)
	}

	files, err := RenderTimongoFiles(topo)
	if err != nil {
		t.Fatal(err)
	}
	hostFiles := files["10.0.1.10"]
	if !strings.Contains(hostFiles.Config, `host = "10.0.1.20"`) {
		t.Fatalf("config missing bound TiDB host:\n%s", hostFiles.Config)
	}
	if !strings.Contains(hostFiles.Service, "ExecStart=/tidb-deploy/timongo-27017/bin/timongo serve -config /tidb-deploy/timongo-27017/conf/timongo.toml") {
		t.Fatalf("service has wrong ExecStart:\n%s", hostFiles.Service)
	}
}
