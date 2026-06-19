package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Garante ambiente limpo.
	for _, k := range []string{"PORT", "LOG_LEVEL", "LOG_FORMAT", "CALC_DELAY", "CACHE_ENABLED", "CACHE_TTL", "REDIS_ADDR", "REDIS_PASSWORD", "REDIS_DB"} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8000 {
		t.Errorf("Port = %d, want 8000", cfg.Port)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", cfg.LogFormat)
	}
	if cfg.CalcDelay != 5*time.Second {
		t.Errorf("CalcDelay = %v, want 5s", cfg.CalcDelay)
	}
	if cfg.Cache.Enabled {
		t.Errorf("Cache.Enabled = true, want false (disabled by default)")
	}
	if cfg.Cache.TTL != 60*time.Second {
		t.Errorf("Cache.TTL = %v, want 60s", cfg.Cache.TTL)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("CALC_DELAY", "100ms")
	t.Setenv("CACHE_ENABLED", "true")
	t.Setenv("CACHE_TTL", "30s")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "2")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.CalcDelay != 100*time.Millisecond {
		t.Errorf("CalcDelay = %v, want 100ms", cfg.CalcDelay)
	}
	if !cfg.Cache.Enabled {
		t.Errorf("Cache.Enabled = false, want true")
	}
	if cfg.Cache.RedisAddr != "redis:6379" {
		t.Errorf("RedisAddr = %q, want redis:6379", cfg.Cache.RedisAddr)
	}
	if cfg.Cache.RedisDB != 2 {
		t.Errorf("RedisDB = %d, want 2", cfg.Cache.RedisDB)
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Setenv("PORT", "70000")
	if _, err := Load(); err == nil {
		t.Errorf("expected error for invalid port, got nil")
	}
}

func TestLoadInvalidValuesFallBackToDefaults(t *testing.T) {
	t.Setenv("CACHE_ENABLED", "notabool")
	t.Setenv("CALC_DELAY", "notaduration")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cache.Enabled {
		t.Errorf("invalid CACHE_ENABLED should fall back to false")
	}
	if cfg.CalcDelay != 5*time.Second {
		t.Errorf("invalid CALC_DELAY should fall back to 5s, got %v", cfg.CalcDelay)
	}
}
