package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/fernandosoaresjr/foxbit-calc/internal/calculator"
	"github.com/fernandosoaresjr/foxbit-calc/internal/observability"
)

// fakeCache é um cache em memória com hooks para forçar erros nos testes.
type fakeCache struct {
	mu      sync.Mutex
	data    map[string]float64
	getErr  error
	setErr  error
	sets    int
}

func newFakeCache() *fakeCache { return &fakeCache{data: map[string]float64{}} }

func (f *fakeCache) Get(_ context.Context, key string) (float64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return 0, false, f.getErr
	}
	v, ok := f.data[key]
	return v, ok, nil
}

func (f *fakeCache) Set(_ context.Context, key string, value float64, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return f.setErr
	}
	f.data[key] = value
	f.sets++
	return nil
}

func (f *fakeCache) Ping(context.Context) error { return nil }
func (f *fakeCache) Close() error               { return nil }

func newTestService(c *fakeCache, delay time.Duration) (*Service, *observability.Metrics) {
	m := observability.NewMetrics(prometheus.NewRegistry())
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(c, m, l, delay, time.Minute), m
}

func intPtr(i int) *int { return &i }

func TestComputeResults(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want float64
	}{
		{"sum integer", Request{calculator.Sum, 4, 1, nil}, 5},
		{"sub integer", Request{calculator.Sub, 4, 1, nil}, 3},
		{"mul integer", Request{calculator.Mul, 4, 3, nil}, 12},
		{"div truncated to int", Request{calculator.Div, 4, 3, nil}, 1},
		{"div precision 2", Request{calculator.Div, 4, 3, intPtr(2)}, 1.33},
		{"div precision 0 explicit", Request{calculator.Div, 10, 4, intPtr(0)}, 2},
		{"div precision 1", Request{calculator.Div, 10, 4, intPtr(1)}, 2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newFakeCache()
			svc, _ := newTestService(c, 0)
			got, err := svc.Compute(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeDivideByZero(t *testing.T) {
	c := newFakeCache()
	svc, _ := newTestService(c, 0)
	_, err := svc.Compute(context.Background(), Request{calculator.Div, 1, 0, nil})
	if !errors.Is(err, calculator.ErrDivisionByZero) {
		t.Fatalf("got %v, want ErrDivisionByZero", err)
	}
	if c.sets != 0 {
		t.Errorf("error result must not be cached, got %d sets", c.sets)
	}
}

func TestComputeMissThenHit(t *testing.T) {
	c := newFakeCache()
	svc, m := newTestService(c, 0)
	req := Request{calculator.Sum, 4, 1, nil}

	// 1ª chamada: miss + set.
	if _, err := svc.Compute(context.Background(), req); err != nil {
		t.Fatalf("compute 1: %v", err)
	}
	// 2ª chamada: hit.
	if _, err := svc.Compute(context.Background(), req); err != nil {
		t.Fatalf("compute 2: %v", err)
	}

	if got := testutil.ToFloat64(m.CacheMisses.WithLabelValues("sum")); got != 1 {
		t.Errorf("misses = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.CacheHits.WithLabelValues("sum")); got != 1 {
		t.Errorf("hits = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.CacheSets.WithLabelValues("sum")); got != 1 {
		t.Errorf("sets = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.Operations.WithLabelValues("sum")); got != 2 {
		t.Errorf("operations = %v, want 2", got)
	}
}

func TestComputeCacheGetErrorTreatedAsMiss(t *testing.T) {
	c := newFakeCache()
	c.getErr = errors.New("boom")
	svc, m := newTestService(c, 0)

	got, err := svc.Compute(context.Background(), Request{calculator.Sum, 2, 2, nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 4 {
		t.Errorf("got %v, want 4", got)
	}
	if n := testutil.ToFloat64(m.CacheErrors); n != 1 {
		t.Errorf("cache errors = %v, want 1", n)
	}
}

func TestComputeCacheSetErrorIsBestEffort(t *testing.T) {
	c := newFakeCache()
	c.setErr = errors.New("boom")
	svc, m := newTestService(c, 0)

	got, err := svc.Compute(context.Background(), Request{calculator.Sum, 2, 2, nil})
	if err != nil {
		t.Fatalf("set error must not fail the request: %v", err)
	}
	if got != 4 {
		t.Errorf("got %v, want 4", got)
	}
	if n := testutil.ToFloat64(m.CacheErrors); n != 1 {
		t.Errorf("cache errors = %v, want 1", n)
	}
}

func TestComputeDelayOnlyOnMiss(t *testing.T) {
	c := newFakeCache()
	// Delay grande; contexto cancelado deve abortar o miss.
	svc, _ := newTestService(c, 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := svc.Compute(ctx, Request{calculator.Sum, 1, 1, nil})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want context deadline exceeded", err)
	}
}
