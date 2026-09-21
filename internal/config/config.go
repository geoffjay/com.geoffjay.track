// Package config loads process configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config carries the process configuration. Everything is settable via
// environment variables with sensible local-dev defaults; TRACK_* variables
// override on Fly via fly.toml [env] or `fly secrets`.
type Config struct {
	// Port is the HTTP listen port (PORT / TRACK_PORT, default 8080).
	Port int
	// DBPath is the SQLite database file path (TRACK_DB_PATH, default ./data/track.db).
	DBPath string
	// DataPath is the directory holding runtime files (TRACK_DATA_PATH, default ./data).
	DataPath string
	// Realm is the basic-auth realm shown by the browser login prompt
	// (TRACK_REALM, default "rowing miles").
	Realm string
	// PasswordCost is the bcrypt cost used when seeding users
	// (TRACK_PASSWORD_COST, default 10; lower it for faster tests).
	PasswordCost int
	// Env labels the deployment (TRACK_ENV, default "development").
	Env string
}

// IsProduction reports whether the app runs with production semantics
// (gin release mode).
func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads the environment and returns the effective Config.
func Load() (Config, error) {
	cfg := Config{
		Port:         envInt("PORT", 8080),
		DBPath:       envString("TRACK_DB_PATH", "data/track.db"),
		DataPath:     envString("TRACK_DATA_PATH", "data"),
		Realm:        envString("TRACK_REALM", "rowing miles"),
		PasswordCost: envInt("TRACK_PASSWORD_COST", 10),
		Env:          envString("TRACK_ENV", "development"),
	}
	if cfg.DBPath == "" {
		return cfg, fmt.Errorf("TRACK_DB_PATH must not be empty")
	}
	return cfg, nil
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// GracefulShutdownTimeout bounds context shutdown handling.
const GracefulShutdownTimeout = 10 * time.Second