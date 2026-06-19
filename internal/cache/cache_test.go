package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestNoopCache(t *testing.T) {
	c := NewNoopCache()
	ctx := context.Background()

	if err := c.Set(ctx, "k", 1.23, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	_, found, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Errorf("NoopCache should never report a hit")
	}
	if err := c.Ping(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func newTestRedis(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	c := NewRedisCache(RedisOptions{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c, mr
}

func TestRedisCacheSetGet(t *testing.T) {
	c, _ := newTestRedis(t)
	ctx := context.Background()

	if _, found, _ := c.Get(ctx, "absent"); found {
		t.Errorf("expected miss for absent key")
	}

	if err := c.Set(ctx, "sum:4:1:int", 5, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, found, err := c.Get(ctx, "sum:4:1:int")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found || val != 5 {
		t.Errorf("got (%v, %v), want (5, true)", val, found)
	}
}

func TestRedisCacheExpiration(t *testing.T) {
	c, mr := newTestRedis(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k", 9, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// Avança o relógio do miniredis além do TTL.
	mr.FastForward(2 * time.Minute)

	if _, found, _ := c.Get(ctx, "k"); found {
		t.Errorf("expected miss after expiration")
	}
}

func TestRedisCachePingFailure(t *testing.T) {
	c, mr := newTestRedis(t)
	mr.Close() // derruba o servidor
	if err := c.Ping(context.Background()); err == nil {
		t.Errorf("expected Ping error when redis is down")
	}
}

func TestRedisCacheFloatRoundTrip(t *testing.T) {
	c, _ := newTestRedis(t)
	ctx := context.Background()
	if err := c.Set(ctx, "div", 1.33, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, found, err := c.Get(ctx, "div")
	if err != nil || !found {
		t.Fatalf("Get: val=%v found=%v err=%v", val, found, err)
	}
	if val != 1.33 {
		t.Errorf("got %v, want 1.33", val)
	}
}
