package config

import (
    "fmt"
    "os"
)

type Config struct {
    Env         string
    ListenAddr  string
    DatabaseURL string
    ScanWorkers int
}

func getenv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}

func Load() (Config, error) {
    cfg := Config{
        Env:         getenv("APP_ENV", "development"),
        ListenAddr:  getenv("LISTEN_ADDR", ":8080"),
        DatabaseURL: os.Getenv("DATABASE_URL"),
        ScanWorkers: getenvInt("SCAN_WORKERS", 0),
    }
    if cfg.DatabaseURL == "" {
        // Not fatal for early local runs; warn via error value so callers can decide.
        return cfg, fmt.Errorf("DATABASE_URL not set")
    }
    return cfg, nil
}

func getenvInt(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        var out int
        _, err := fmt.Sscanf(v, "%d", &out)
        if err == nil { return out }
    }
    return def
}
