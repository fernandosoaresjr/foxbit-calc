// Package config carrega a configuração da aplicação a partir de variáveis de
// ambiente, aplicando defaults sensatos. Nenhuma variável é obrigatória: a
// aplicação sobe com defaults e degrada graciosamente (ex.: sem cache).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config agrega toda a configuração da aplicação.
type Config struct {
	// Port é a porta HTTP do servidor (default 8000, conforme o desafio).
	Port int
	// LogLevel: debug, info, warn, error (default info).
	LogLevel string
	// LogFormat: json ou text (default json).
	LogFormat string
	// CalcDelay simula o custo de uma operação cara; aplicado em cache miss.
	CalcDelay time.Duration

	// Cache agrupa a configuração de cache.
	Cache CacheConfig
}

// CacheConfig agrupa a configuração do cache Redis.
type CacheConfig struct {
	// Enabled liga/desliga o cache. Default false (desabilitado).
	Enabled bool
	// TTL é o tempo de expiração de cada entrada (default 60s).
	TTL time.Duration
	// RedisAddr é o endereço host:port do Redis.
	RedisAddr string
	// RedisPassword é a senha do Redis (vazia = sem auth).
	RedisPassword string
	// RedisDB é o índice do banco Redis (default 0).
	RedisDB int
}

// Load lê a configuração das variáveis de ambiente.
func Load() (Config, error) {
	cfg := Config{
		Port:      getInt("PORT", 8000),
		LogLevel:  getString("LOG_LEVEL", "info"),
		LogFormat: getString("LOG_FORMAT", "json"),
		CalcDelay: getDuration("CALC_DELAY", 5*time.Second),
		Cache: CacheConfig{
			Enabled:       getBool("CACHE_ENABLED", false),
			TTL:           getDuration("CACHE_TTL", 60*time.Second),
			RedisAddr:     getString("REDIS_ADDR", ""),
			RedisPassword: getString("REDIS_PASSWORD", ""),
			RedisDB:       getInt("REDIS_DB", 0),
		},
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("invalid PORT: %d", cfg.Port)
	}
	return cfg, nil
}

func getString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
