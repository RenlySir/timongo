package config

import "time"

// Config contains timongo runtime settings.
type Config struct {
	ListenAddr string
	Backend    string
	TiDBDSN    string
	Timeout    time.Duration
}

// Default returns default local-development settings.
func Default() Config {
	return Config{
		ListenAddr: "127.0.0.1:27017",
		Backend:    "memory",
		Timeout:    30 * time.Second,
	}
}
