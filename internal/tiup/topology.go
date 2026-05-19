package tiup

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

var timongoSectionNames = map[string]struct{}{
	"timongo_servers":    {},
	"timongo_lb_servers": {},
}

// Topology describes timongo-related TiUP topology sections.
type Topology struct {
	Global           Global            `yaml:"global"`
	TimongoServers   []TimongoServer   `yaml:"timongo_servers"`
	TimongoLBServers []TimongoLBServer `yaml:"timongo_lb_servers"`
}

// Global contains topology-wide values reused by timongo deployment helpers.
type Global struct {
	User      string `yaml:"user"`
	DeployDir string `yaml:"deploy_dir"`
	DataDir   string `yaml:"data_dir"`
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

// SplitTopology removes timongo-only sections before passing a topology to tiup cluster.
func SplitTopology(raw []byte) ([]byte, []byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, nil, err
	}
	if len(root.Content) == 0 {
		return raw, []byte("{}\n"), nil
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("topology root must be a YAML mapping")
	}

	tidbDoc := &yaml.Node{Kind: yaml.MappingNode}
	timongoDoc := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i < len(doc.Content); i += 2 {
		key := doc.Content[i]
		value := doc.Content[i+1]
		if _, ok := timongoSectionNames[key.Value]; ok {
			timongoDoc.Content = append(timongoDoc.Content, cloneNode(key), cloneNode(value))
			continue
		}
		tidbDoc.Content = append(tidbDoc.Content, cloneNode(key), cloneNode(value))
		if key.Value == "global" {
			timongoDoc.Content = append(timongoDoc.Content, cloneNode(key), cloneNode(value))
		}
	}

	tidbRaw, err := yaml.Marshal(tidbDoc)
	if err != nil {
		return nil, nil, err
	}
	timongoRaw, err := yaml.Marshal(timongoDoc)
	if err != nil {
		return nil, nil, err
	}
	return tidbRaw, timongoRaw, nil
}

func cloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	cp := *n
	cp.Content = make([]*yaml.Node, len(n.Content))
	for i, child := range n.Content {
		cp.Content[i] = cloneNode(child)
	}
	return &cp
}
