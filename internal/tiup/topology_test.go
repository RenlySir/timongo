package tiup

import "testing"

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
