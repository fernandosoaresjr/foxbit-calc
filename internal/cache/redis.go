package cache

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implementa Cache sobre um cliente Redis (go-redis v9).
type RedisCache struct {
	client *redis.Client
}

// RedisOptions agrupa os parâmetros de conexão ao Redis.
type RedisOptions struct {
	Addr     string
	Password string
	DB       int
}

// NewRedisCache cria um RedisCache. Não conecta de imediato — use Ping para
// verificar a disponibilidade.
func NewRedisCache(opts RedisOptions) *RedisCache {
	return &RedisCache{
		client: redis.NewClient(&redis.Options{
			Addr:     opts.Addr,
			Password: opts.Password,
			DB:       opts.DB,
		}),
	}
}

// Get busca key no Redis. Retorna found=false em chave inexistente/expirada.
func (c *RedisCache) Get(ctx context.Context, key string) (float64, bool, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		// Valor corrompido: trata como miss para não derrubar a requisição.
		return 0, false, err
	}
	return f, true, nil
}

// Set grava value sob key com expiração ttl.
func (c *RedisCache) Set(ctx context.Context, key string, value float64, ttl time.Duration) error {
	return c.client.Set(ctx, key, strconv.FormatFloat(value, 'f', -1, 64), ttl).Err()
}

// Ping verifica a conectividade com o Redis.
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close fecha o cliente Redis.
func (c *RedisCache) Close() error {
	return c.client.Close()
}
