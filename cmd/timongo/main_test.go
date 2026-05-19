package main

import (
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
