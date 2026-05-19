package config

import "time"

const (
	// BackendTiDB is the only runtime backend. It keeps timongo stateless by storing all data in TiDB.
	BackendTiDB = "tidb"
)

// Config contains timongo runtime settings.
type Config struct {
	ListenAddr    string
	StatusAddr    string
	Backend       string
	TiDBDSN       string
	CompatVersion string
	Timeout       time.Duration
}

// Default returns production-safe local defaults.
func Default() Config {
	return Config{
		ListenAddr:    "127.0.0.1:27017",
		StatusAddr:    "127.0.0.1:28017",
		Backend:       BackendTiDB,
		CompatVersion: "6.0",
		Timeout:       30 * time.Second,
	}
}
