package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics agrega os coletores Prometheus da aplicação. As métricas de cache são
// rotuladas por operação (cardinalidade baixa: sum/sub/mul/div) — os valores dos
// termos NÃO entram como rótulo para evitar explosão de cardinalidade.
type Metrics struct {
	Operations   *prometheus.CounterVec   // total de operações por tipo
	CacheHits    *prometheus.CounterVec   // acertos de cache por operação
	CacheMisses  *prometheus.CounterVec   // perdas de cache por operação
	CacheSets    *prometheus.CounterVec   // gravações de cache por operação
	CacheErrors  prometheus.Counter       // erros de acesso ao cache (geral)
	HTTPRequests *prometheus.CounterVec   // requisições HTTP por método/rota/status
	HTTPDuration *prometheus.HistogramVec // latência HTTP por método/rota
}

// NewMetrics registra e devolve os coletores no Registerer informado.
// Receber o Registerer (em vez de usar o default global) torna o pacote
// testável e evita pânico por registro duplicado.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		Operations: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "calc_operations_total",
			Help: "Número total de operações executadas, por tipo de operação.",
		}, []string{"operation"}),
		CacheHits: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "calc_cache_hits_total",
			Help: "Número total de acertos de cache, por operação.",
		}, []string{"operation"}),
		CacheMisses: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "calc_cache_misses_total",
			Help: "Número total de perdas de cache, por operação.",
		}, []string{"operation"}),
		CacheSets: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "calc_cache_sets_total",
			Help: "Número total de gravações de cache, por operação.",
		}, []string{"operation"}),
		CacheErrors: factory.NewCounter(prometheus.CounterOpts{
			Name: "calc_cache_errors_total",
			Help: "Número total de erros ao acessar o cache.",
		}),
		HTTPRequests: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "calc_http_requests_total",
			Help: "Número total de requisições HTTP, por método, rota e status.",
		}, []string{"method", "route", "status"}),
		HTTPDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "calc_http_request_duration_seconds",
			Help:    "Duração das requisições HTTP em segundos, por método e rota.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
	}
}
