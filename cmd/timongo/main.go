package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/RenlySir/timongo/internal/backend"
	tidbbackend "github.com/RenlySir/timongo/internal/backend/tidb"
	"github.com/RenlySir/timongo/internal/config"
	"github.com/RenlySir/timongo/internal/handler"
	"github.com/RenlySir/timongo/internal/status"
	"github.com/RenlySir/timongo/internal/tiup"
	wireserver "github.com/RenlySir/timongo/internal/wire"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		if err := serve(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "tiup":
		if err := tiupCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func serve(args []string) error {
	cfg, err := parseServeConfig(args)
	if err != nil {
		return err
	}

	store, closeStore, err := buildStore(cfg)
	if err != nil {
		return err
	}
	defer closeStore()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ready := func() error { return nil }
	if r, ok := store.(interface{ Ready() error }); ok {
		ready = r.Ready
	}
	go func() {
		if err := status.ListenAndServe(ctx, cfg.StatusAddr, status.NewHandler(ready)); err != nil {
			log.Printf("status server stopped: %v", err)
		}
	}()

	server := &wireserver.Server{
		Addr:    cfg.ListenAddr,
		Handler: handler.New(store),
	}

	log.Printf("timongo listening on %s with %s backend", cfg.ListenAddr, cfg.Backend)
	return server.ListenAndServe(ctx)
}

func parseServeConfig(args []string) (config.Config, error) {
	defaults := config.Default()
	var configPath string
	var listenAddr string
	var statusAddr string
	var backendName string
	var tidbDSN string

	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "", "TOML config file rendered by TiUP")
	fs.StringVar(&listenAddr, "listen", defaults.ListenAddr, "MongoDB wire protocol listen address")
	fs.StringVar(&statusAddr, "status-listen", defaults.StatusAddr, "HTTP health and metrics listen address")
	fs.StringVar(&backendName, "backend", defaults.Backend, "storage backend: tidb")
	fs.StringVar(&tidbDSN, "tidb-dsn", defaults.TiDBDSN, "TiDB/MySQL DSN, for example user:pass@tcp(127.0.0.1:4000)/timongo?parseTime=true")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, err
	}

	cfg := defaults
	if configPath != "" {
		loaded, err := config.LoadFile(configPath)
		if err != nil {
			return config.Config{}, err
		}
		cfg = loaded
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "listen":
			cfg.ListenAddr = listenAddr
		case "status-listen":
			cfg.StatusAddr = statusAddr
		case "backend":
			cfg.Backend = backendName
		case "tidb-dsn":
			cfg.TiDBDSN = tidbDSN
		}
	})
	return cfg, nil
}

func buildStore(cfg config.Config) (backend.Store, func(), error) {
	switch cfg.Backend {
	case config.BackendTiDB:
		if cfg.TiDBDSN == "" {
			return nil, nil, fmt.Errorf("-tidb-dsn is required when -backend=tidb")
		}
		store, err := tidbbackend.NewStore(cfg.TiDBDSN)
		if err != nil {
			return nil, nil, err
		}
		return store, func() { _ = store.Close() }, nil
	case "memory":
		return nil, nil, fmt.Errorf("memory backend is test-only; timongo runtime must be stateless and use tidb")
	default:
		return nil, nil, fmt.Errorf("unsupported backend %q", cfg.Backend)
	}
}

func tiupCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: timongo tiup <split-topology|render-timongo> [flags]")
	}
	switch args[0] {
	case "split-topology":
		return runTiUPSplitTopology(args[1:])
	case "render-timongo":
		return runTiUPRenderTimongo(args[1:])
	default:
		return fmt.Errorf("unknown tiup subcommand %q", args[0])
	}
}

func runTiUPSplitTopology(args []string) error {
	var input string
	var tidbOutput string
	var timongoOutput string

	fs := flag.NewFlagSet("tiup split-topology", flag.ExitOnError)
	fs.StringVar(&input, "input", "", "topology.yaml with timongo sections")
	fs.StringVar(&tidbOutput, "tidb-output", "", "output topology.yaml for tiup cluster")
	fs.StringVar(&timongoOutput, "timongo-output", "", "output topology.yaml with timongo sections")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if input == "" || tidbOutput == "" || timongoOutput == "" {
		return fmt.Errorf("-input, -tidb-output, and -timongo-output are required")
	}

	raw, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	tidbRaw, timongoRaw, err := tiup.SplitTopology(raw)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tidbOutput, tidbRaw, 0o600); err != nil {
		return err
	}
	return os.WriteFile(timongoOutput, timongoRaw, 0o600)
}

func runTiUPRenderTimongo(args []string) error {
	var input string
	var output string

	fs := flag.NewFlagSet("tiup render-timongo", flag.ExitOnError)
	fs.StringVar(&input, "input", "", "topology.yaml with timongo sections")
	fs.StringVar(&output, "output", "", "rendered output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if input == "" || output == "" {
		return fmt.Errorf("-input and -output are required")
	}

	raw, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	topo, err := tiup.ParseTopology(raw)
	if err != nil {
		return err
	}
	files, err := tiup.RenderTimongoFiles(topo)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	var hosts []byte
	for host, rendered := range files {
		hostDir := filepath.Join(output, host)
		if err := os.MkdirAll(hostDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(hostDir, "timongo.toml"), []byte(rendered.Config), 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(hostDir, "timongo.service"), []byte(rendered.Service), 0o600); err != nil {
			return err
		}
		hosts = append(hosts, []byte(fmt.Sprintf("%s\t%s\t%s\n", host, rendered.DeployDir, rendered.LogDir))...)
	}
	return os.WriteFile(filepath.Join(output, "hosts.tsv"), hosts, 0o600)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: timongo serve [flags]\n       timongo tiup <split-topology|render-timongo> [flags]\n")
}
