package tiup

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

//go:embed templates/timongo.toml.tmpl
var timongoConfigTemplate string

//go:embed templates/timongo.service.tmpl
var timongoServiceTemplate string

// RenderedFiles contains per-host files for timongo deployment.
type RenderedFiles struct {
	Config    string
	Service   string
	DeployDir string
	LogDir    string
}

// RenderTimongoFiles renders timongo config and systemd files from topology.
func RenderTimongoFiles(topo *Topology) (map[string]RenderedFiles, error) {
	if topo == nil {
		return nil, fmt.Errorf("topology is nil")
	}
	user := topo.Global.User
	if user == "" {
		user = "tidb"
	}

	configTmpl, err := template.New("timongo.toml").Parse(timongoConfigTemplate)
	if err != nil {
		return nil, err
	}
	serviceTmpl, err := template.New("timongo.service").Parse(timongoServiceTemplate)
	if err != nil {
		return nil, err
	}

	files := make(map[string]RenderedFiles, len(topo.TimongoServers))
	for _, server := range topo.TimongoServers {
		normalizeServerDefaults(&server)
		data := struct {
			User string
			TimongoServer
		}{
			User:          user,
			TimongoServer: server,
		}

		var config bytes.Buffer
		if err := configTmpl.Execute(&config, data); err != nil {
			return nil, err
		}
		var service bytes.Buffer
		if err := serviceTmpl.Execute(&service, data); err != nil {
			return nil, err
		}
		files[server.Host] = RenderedFiles{
			Config:    config.String(),
			Service:   service.String(),
			DeployDir: server.DeployDir,
			LogDir:    server.LogDir,
		}
	}
	return files, nil
}

func normalizeServerDefaults(server *TimongoServer) {
	if server.Port == 0 {
		server.Port = 27017
	}
	if server.StatusPort == 0 {
		server.StatusPort = 28017
	}
	if server.DeployDir == "" {
		server.DeployDir = fmt.Sprintf("/tidb-deploy/timongo-%d", server.Port)
	}
	if server.LogDir == "" {
		server.LogDir = server.DeployDir + "/log"
	}
	if server.Config.CompatVersion == "" {
		server.Config.CompatVersion = "6.0"
	}
	if server.Config.CursorTTL == "" {
		server.Config.CursorTTL = "10m"
	}
	if server.Config.SessionTTL == "" {
		server.Config.SessionTTL = "30m"
	}
	if server.Config.TransactionTimeout == "" {
		server.Config.TransactionTimeout = "60s"
	}
	if server.Config.TiDBMaxOpenConns == 0 {
		server.Config.TiDBMaxOpenConns = 4096
	}
}
