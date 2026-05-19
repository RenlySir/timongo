package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type fileConfig struct {
	Mongo   mongoConfig   `toml:"mongo"`
	Status  statusConfig  `toml:"status"`
	TiDB    tiDBConfig    `toml:"tidb"`
	Runtime runtimeConfig `toml:"runtime"`
}

type mongoConfig struct {
	ListenAddr    string `toml:"listen_addr"`
	CompatVersion string `toml:"compat_version"`
}

type statusConfig struct {
	ListenAddr string `toml:"listen_addr"`
}

type tiDBConfig struct {
	Host         string `toml:"host"`
	Port         int    `toml:"port"`
	Database     string `toml:"database"`
	User         string `toml:"user"`
	Password     string `toml:"password"`
	DSN          string `toml:"dsn"`
	MaxOpenConns int    `toml:"max_open_conns"`
}

type runtimeConfig struct {
	Stateless          *bool  `toml:"stateless"`
	CursorTTL          string `toml:"cursor_ttl"`
	SessionTTL         string `toml:"session_ttl"`
	TransactionTimeout string `toml:"transaction_timeout"`
}

// LoadFile reads a TiUP-rendered timongo TOML config file.
func LoadFile(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	cfg := Default()
	var fileCfg fileConfig
	if err := toml.Unmarshal(raw, &fileCfg); err != nil {
		return Config{}, err
	}
	if fileCfg.Mongo.ListenAddr != "" {
		cfg.ListenAddr = fileCfg.Mongo.ListenAddr
	}
	if fileCfg.Mongo.CompatVersion != "" {
		cfg.CompatVersion = fileCfg.Mongo.CompatVersion
	}
	if fileCfg.Status.ListenAddr != "" {
		cfg.StatusAddr = fileCfg.Status.ListenAddr
	}
	if fileCfg.Runtime.Stateless != nil && !*fileCfg.Runtime.Stateless {
		return Config{}, fmt.Errorf("timongo-server must run stateless=true")
	}

	dsn, err := buildTiDBDSN(fileCfg.TiDB)
	if err != nil {
		return Config{}, err
	}
	if dsn != "" {
		cfg.TiDBDSN = dsn
	}
	return cfg, nil
}

func buildTiDBDSN(cfg tiDBConfig) (string, error) {
	if cfg.DSN != "" {
		return cfg.DSN, nil
	}
	if cfg.Host == "" && cfg.Port == 0 {
		return "", nil
	}
	if cfg.Host == "" {
		return "", fmt.Errorf("tidb.host is required when tidb config is provided")
	}
	if cfg.Port == 0 {
		return "", fmt.Errorf("tidb.port is required when tidb config is provided")
	}

	database := cfg.Database
	if database == "" {
		database = "_timongo"
	}
	user := cfg.User
	if user == "" {
		user = "root"
	}

	auth := user
	if cfg.Password != "" {
		auth += ":" + url.QueryEscape(cfg.Password)
	}
	return fmt.Sprintf("%s@tcp(%s:%d)/%s?parseTime=true", auth, cfg.Host, cfg.Port, database), nil
}
