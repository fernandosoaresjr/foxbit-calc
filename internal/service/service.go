// Package service orquestra o cálculo de uma operação: aplica o delay que
// simula uma operação custosa, consulta/popula o cache e emite métricas e logs.
// É a cola entre o núcleo puro (calculator) e a borda (handlers HTTP).
package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/fernandosoaresjr/foxbit-calc/internal/cache"
	"github.com/fernandosoaresjr/foxbit-calc/internal/calculator"
	"github.com/fernandosoaresjr/foxbit-calc/internal/observability"
)

// Request descreve um pedido de cálculo. Precision nil significa "resultado
// inteiro truncado".
type Request struct {
	Operation calculator.Operation
	TermOne   float64
	TermTwo   float64
	Precision *int
}

// Service executa operações com cache e instrumentação.
type Service struct {
	cache   cache.Cache
	metrics *observability.Metrics
	logger  *slog.Logger
	delay   time.Duration
	ttl     time.Duration
}

// New cria um Service. cache nunca deve ser nil (use cache.NewNoopCache() para
// desabilitar). metrics e logger também não devem ser nil.
func New(c cache.Cache, m *observability.Metrics, l *slog.Logger, delay, ttl time.Duration) *Service {
	return &Service{cache: c, metrics: m, logger: l, delay: delay, ttl: ttl}
}

// Compute resolve a operação, usando cache quando possível. O delay (operação
// custosa simulada) só ocorre em cache miss.
func (s *Service) Compute(ctx context.Context, req Request) (float64, error) {
	op := string(req.Operation)
	key := cacheKey(req)
	s.metrics.Operations.WithLabelValues(op).Inc()

	// 1. Tenta o cache.
	if val, found, err := s.cache.Get(ctx, key); err != nil {
		// Erro real de acesso: conta, loga e segue como miss (degradação).
		s.metrics.CacheErrors.Inc()
		s.logger.Error("cache get failed", "operation", op, "key", key, "error", err)
	} else if found {
		s.metrics.CacheHits.WithLabelValues(op).Inc()
		s.logger.Info("cache hit",
			"operation", op, "term_one", req.TermOne, "term_two", req.TermTwo,
			"precision", precisionLabel(req.Precision), "result", val)
		return val, nil
	}

	// 2. Miss: registra, simula o custo e calcula.
	s.metrics.CacheMisses.WithLabelValues(op).Inc()
	s.logger.Info("cache miss",
		"operation", op, "term_one", req.TermOne, "term_two", req.TermTwo,
		"precision", precisionLabel(req.Precision))

	if err := s.sleep(ctx, s.delay); err != nil {
		return 0, err
	}

	raw, err := calculator.Calculate(req.Operation, req.TermOne, req.TermTwo)
	if err != nil {
		return 0, err
	}
	result := calculator.Truncate(raw, effectivePrecision(req.Precision))

	// 3. Popula o cache (best-effort: falha não derruba a requisição).
	if err := s.cache.Set(ctx, key, result, s.ttl); err != nil {
		s.metrics.CacheErrors.Inc()
		s.logger.Error("cache set failed", "operation", op, "key", key, "error", err)
	} else {
		s.metrics.CacheSets.WithLabelValues(op).Inc()
		s.logger.Info("cache updated",
			"operation", op, "term_one", req.TermOne, "term_two", req.TermTwo,
			"precision", precisionLabel(req.Precision), "result", result)
	}

	return result, nil
}

// sleep aguarda d respeitando o cancelamento do contexto.
func (s *Service) sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// cacheKey monta uma chave estável: <op>:<term_one>:<term_two>:<precision>.
func cacheKey(req Request) string {
	return string(req.Operation) + ":" +
		strconv.FormatFloat(req.TermOne, 'f', -1, 64) + ":" +
		strconv.FormatFloat(req.TermTwo, 'f', -1, 64) + ":" +
		precisionLabel(req.Precision)
}

// precisionLabel devolve "int" para precisão nil ou o número de casas.
func precisionLabel(p *int) string {
	if p == nil {
		return "int"
	}
	return strconv.Itoa(*p)
}

// effectivePrecision converte precisão nil em 0 (inteiro).
func effectivePrecision(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
