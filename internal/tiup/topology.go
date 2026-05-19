package tiup

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Topology describes timongo-related TiUP topology sections.
type Topology struct {
	TimongoServers   []TimongoServer   `yaml:"timongo_servers"`
	TimongoLBServers []TimongoLBServer `yaml:"timongo_lb_servers"`
}

// TimongoServer describes one timongo-server instance.
type TimongoServer struct {
	Host       string        `yaml:"host"`
	Port       int           `yaml:"port"`
	StatusPort int           `yaml:"status_port"`
	DeployDir  string        `yaml:"deploy_dir"`
	LogDir     string        `yaml:"log_dir"`
	TiDBHost   string        `yaml:"tidb_host"`
	TiDBPort   int           `yaml:"tidb_port"`
	Config     TimongoConfig `yaml:"config"`
}

// TimongoConfig contains per-instance runtime config.
type TimongoConfig struct {
	CompatVersion      string `yaml:"compat_version"`
	Stateless          bool   `yaml:"stateless"`
	CursorTTL          string `yaml:"cursor_ttl"`
	SessionTTL         string `yaml:"session_ttl"`
	TransactionTimeout string `yaml:"transaction_timeout"`
	MaxConnections     int    `yaml:"max_connections"`
	TiDBMaxOpenConns   int    `yaml:"tidb_max_open_conns"`
}

// TimongoLBServer describes optional HAProxy/NLB management metadata.
type TimongoLBServer struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	StatusPort  int    `yaml:"status_port"`
	Balance     string `yaml:"balance"`
	BackendRole string `yaml:"backend_role"`
}

// ParseTopology parses and validates timongo topology sections.
func ParseTopology(raw []byte) (*Topology, error) {
	var topo Topology
	if err := yaml.Unmarshal(raw, &topo); err != nil {
		return nil, err
	}
	for i, server := range topo.TimongoServers {
		if server.Host == "" {
			return nil, fmt.Errorf("timongo_servers[%d].host is required", i)
		}
		if server.Port == 0 {
			return nil, fmt.Errorf("timongo_servers[%d].port is required", i)
		}
		if server.StatusPort == 0 {
			return nil, fmt.Errorf("timongo_servers[%d].status_port is required", i)
		}
		if server.TiDBHost == "" {
			return nil, fmt.Errorf("timongo_servers[%d].tidb_host is required", i)
		}
		if server.TiDBPort == 0 {
			return nil, fmt.Errorf("timongo_servers[%d].tidb_port is required", i)
		}
	}
	return &topo, nil
}
