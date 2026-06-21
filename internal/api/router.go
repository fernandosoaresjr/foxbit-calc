package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fernandosoaresjr/foxbit-calc/internal/observability"
)

// NewRouter monta o *echo.Echo com middlewares, endpoints operacionais
// (health/metrics, sem o delay) e os endpoints de cálculo gerados do contrato.
func NewRouter(server *Server, reg *prometheus.Registry, metrics *observability.Metrics, logger *slog.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(observe(logger, metrics))

	// Endpoints operacionais — não fazem parte do contrato OpenAPI e não passam
	// pelo delay simulado. /readyz NÃO depende do Redis (app degrada sem cache).
	e.GET("/healthz", healthz)
	e.GET("/readyz", readyz)
	e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))

	// Endpoints de cálculo gerados a partir do contrato (api.gen.go).
	RegisterHandlers(e, server)
	return e
}

// healthz é a liveness probe: o processo está vivo.
func healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// readyz é a readiness probe: pronto para receber tráfego. Intencionalmente
// independente do Redis — a aplicação funciona (degradada) sem cache.
func readyz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
}

// observe é um único middleware que loga e instrumenta cada requisição. Trata
// o erro retornado pelo handler uma única vez (c.Error commita a resposta, o que
// finaliza o status), registra log/métricas e devolve nil para evitar que o
// Echo processe o erro novamente.
func observe(logger *slog.Logger, m *observability.Metrics) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			if err := next(c); err != nil {
				c.Error(err)
			}

			route := c.Path()
			if route == "" {
				route = "unknown"
			}
			status := c.Response().Status
			elapsed := time.Since(start)

			m.HTTPRequests.WithLabelValues(c.Request().Method, route, strconv.Itoa(status)).Inc()
			m.HTTPDuration.WithLabelValues(c.Request().Method, route).Observe(elapsed.Seconds())

			// Endpoints operacionais (probes/scrape) são consultados o tempo todo
			// pelo Kubernetes/Prometheus; logamos em DEBUG para não poluir o log.
			level := slog.LevelInfo
			switch route {
			case "/healthz", "/readyz", "/metrics":
				level = slog.LevelDebug
			}
			logger.Log(c.Request().Context(), level, "http request",
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"route", route,
				"status", status,
				"duration_ms", elapsed.Milliseconds(),
			)
			return nil
		}
	}
}
