package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/RenlySir/timongo/internal/backend"
	tidbbackend "github.com/RenlySir/timongo/internal/backend/tidb"
	"github.com/RenlySir/timongo/internal/config"
	"github.com/RenlySir/timongo/internal/handler"
	"github.com/RenlySir/timongo/internal/status"
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
	default:
		usage()
		os.Exit(2)
	}
}

func serve(args []string) error {
	cfg := config.Default()

	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "MongoDB wire protocol listen address")
	fs.StringVar(&cfg.StatusAddr, "status-listen", cfg.StatusAddr, "HTTP health and metrics listen address")
	fs.StringVar(&cfg.Backend, "backend", cfg.Backend, "storage backend: tidb")
	fs.StringVar(&cfg.TiDBDSN, "tidb-dsn", cfg.TiDBDSN, "TiDB/MySQL DSN, for example user:pass@tcp(127.0.0.1:4000)/timongo?parseTime=true")
	if err := fs.Parse(args); err != nil {
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

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: timongo serve [flags]\n")
}
