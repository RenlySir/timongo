package config

import "testing"

func TestDefaultConfigUsesStatelessTiDBBackend(t *testing.T) {
	cfg := Default()

	if cfg.Backend != BackendTiDB {
		t.Fatalf("Backend = %q, want %q", cfg.Backend, BackendTiDB)
	}
}
