// Package cache define a abstração de cache da aplicação e duas implementações:
// RedisCache (Redis real) e NoopCache (no-op, usado quando o cache está
// desabilitado ou o Redis está indisponível — degradação graciosa).
package cache

import (
	"context"
	"time"
)

// Cache é a abstração de cache usada pelo service. Os valores são float64
// (resultados de operações já truncados pela precisão solicitada).
type Cache interface {
	// Get retorna o valor associado a key. found=false quando a chave não existe
	// (ou expirou). err != nil apenas em falhas reais de acesso ao backend.
	Get(ctx context.Context, key string) (value float64, found bool, err error)
	// Set grava value sob key com expiração ttl.
	Set(ctx context.Context, key string, value float64, ttl time.Duration) error
	// Ping verifica a disponibilidade do backend.
	Ping(ctx context.Context) error
	// Close libera recursos do backend.
	Close() error
}

// NoopCache é uma implementação que nunca armazena nada: todo Get é um miss.
// É o fallback quando o cache está desabilitado ou o Redis está inacessível.
type NoopCache struct{}

// NewNoopCache cria um NoopCache.
func NewNoopCache() *NoopCache { return &NoopCache{} }

func (NoopCache) Get(context.Context, string) (float64, bool, error) { return 0, false, nil }
func (NoopCache) Set(context.Context, string, float64, time.Duration) error { return nil }
func (NoopCache) Ping(context.Context) error                                { return nil }
func (NoopCache) Close() error                                              { return nil }
