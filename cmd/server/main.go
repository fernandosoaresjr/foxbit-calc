// Command server sobe a API de calculadora: carrega configuração, inicializa
// logging/métricas/cache (com degradação graciosa) e serve o HTTP na porta
// configurada (default 8000), com shutdown gracioso.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/fernandosoaresjr/foxbit-calc/internal/api"
	"github.com/fernandosoaresjr/foxbit-calc/internal/cache"
	"github.com/fernandosoaresjr/foxbit-calc/internal/config"
	"github.com/fernandosoaresjr/foxbit-calc/internal/observability"
	"github.com/fernandosoaresjr/foxbit-calc/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := observability.NewLogger(cfg.LogLevel, cfg.LogFormat)

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	metrics := observability.NewMetrics(reg)

	c := initCache(cfg, logger)
	defer func() { _ = c.Close() }()

	svc := service.New(c, metrics, logger, cfg.CalcDelay, cfg.Cache.TTL)
	router := api.NewRouter(api.NewServer(svc), reg, metrics, logger)

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("starting server",
		"addr", addr, "calc_delay", cfg.CalcDelay.String(), "log_format", cfg.LogFormat)

	// Sobe o servidor e aguarda sinal de término para shutdown gracioso.
	go func() {
		if err := router.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return router.Shutdown(ctx)
}

// initCache aplica a lógica de degradação graciosa do cache e loga o status
// final na inicialização:
//   - cache desabilitado            -> NoopCache
//   - habilitado sem REDIS_ADDR     -> erro logado, NoopCache
//   - habilitado e Redis acessível  -> RedisCache (conectado)
//   - habilitado mas Redis offline  -> warning logado, RedisCache mesmo assim:
//     o go-redis reconecta sob demanda e o service trata erro de Redis como
//     cache miss, então o cache passa a funcionar assim que o Redis ficar
//     acessível — SEM precisar reiniciar a aplicação. Isso evita ficar preso em
//     "sem cache" quando a app sobe antes do Redis (corrida comum no Kubernetes).
func initCache(cfg config.Config, logger *slog.Logger) cache.Cache {
	if !cfg.Cache.Enabled {
		logger.Info("cache status: disabled (CACHE_ENABLED is false)")
		return cache.NewNoopCache()
	}

	if cfg.Cache.RedisAddr == "" {
		logger.Error("cache status: enabled but REDIS_ADDR is empty; continuing WITHOUT cache")
		return cache.NewNoopCache()
	}

	rc := cache.NewRedisCache(cache.RedisOptions{
		Addr:     cfg.Cache.RedisAddr,
		Password: cfg.Cache.RedisPassword,
		DB:       cfg.Cache.RedisDB,
	})

	// O Redis pode subir junto com a aplicação; tentamos conectar por alguns
	// segundos para logar um status limpo de inicialização.
	var pingErr error
	for attempt := 1; attempt <= 10; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		pingErr = rc.Ping(ctx)
		cancel()
		if pingErr == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if pingErr != nil {
		// Não travamos em "sem cache": mantemos o RedisCache, que se recupera
		// sozinho quando o Redis ficar acessível (ver doc acima).
		logger.Warn("cache status: enabled but Redis not reachable at startup; "+
			"continuing and reconnecting on demand",
			"redis_addr", cfg.Cache.RedisAddr, "error", pingErr)
		return rc
	}

	logger.Info("cache status: enabled and connected",
		"redis_addr", cfg.Cache.RedisAddr, "ttl", cfg.Cache.TTL.String())
	return rc
}
