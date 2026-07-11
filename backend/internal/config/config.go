// Package config loads all runtime configuration from environment variables.
// Every tunable knob in Memory Drive is controlled here so the application can
// be reconfigured at deploy time without rebuilding the image.
package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime settings for the application.
type Config struct {
	// Server
	Port    string
	GinMode string

	// Persistence
	DBPath      string // SQLite database file (lives on the mounted volume)
	UploadDir   string // Directory where uploaded blobs are stored
	MaxUploadMB int    // Maximum size of a single upload in megabytes

	// Memory cache (workload generation)
	EnableMemoryCache bool
	CacheSizeMB       int

	// Background workers (workload generation)
	EnableBackgroundWorkers bool
	WorkerCount             int
	WorkerInterval          time.Duration // how often the periodic job fires

	// Baseline memory allocated at startup, useful for observing a stable
	// memory floor in Grafana without calling /simulate/memory.
	BaselineMemoryMB int
}

// Load reads configuration from the environment, applying sensible defaults.
func Load() *Config {
	cfg := &Config{
		Port:                    getString("PORT", "8080"),
		GinMode:                 getString("GIN_MODE", "release"),
		DBPath:                  getString("DB_PATH", "/data/memorydrive.db"),
		UploadDir:               getString("UPLOAD_DIR", "/data/uploads"),
		MaxUploadMB:             getInt("MAX_UPLOAD_MB", 10),
		EnableMemoryCache:       getBool("ENABLE_MEMORY_CACHE", false),
		CacheSizeMB:             getInt("CACHE_SIZE_MB", 0),
		EnableBackgroundWorkers: getBool("ENABLE_BACKGROUND_WORKERS", false),
		WorkerCount:             getInt("WORKER_COUNT", 2),
		WorkerInterval:          time.Duration(getInt("WORKER_INTERVAL_SECONDS", 10)) * time.Second,
		BaselineMemoryMB:        getInt("BASELINE_MEMORY_MB", 0),
	}

	log.Printf("config loaded: port=%s db=%s uploads=%s cache=%v(%dMB) workers=%v(%d) baseline=%dMB",
		cfg.Port, cfg.DBPath, cfg.UploadDir, cfg.EnableMemoryCache, cfg.CacheSizeMB,
		cfg.EnableBackgroundWorkers, cfg.WorkerCount, cfg.BaselineMemoryMB)

	return cfg
}

func getString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("config: invalid int for %s=%q, using default %d", key, v, fallback)
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		log.Printf("config: invalid bool for %s=%q, using default %v", key, v, fallback)
	}
	return fallback
}
