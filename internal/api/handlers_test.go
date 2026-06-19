package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/fernandosoaresjr/foxbit-calc/internal/cache"
	"github.com/fernandosoaresjr/foxbit-calc/internal/observability"
	"github.com/fernandosoaresjr/foxbit-calc/internal/service"
)

func newTestRouter() *echo.Echo {
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(cache.NewNoopCache(), metrics, logger, 0, time.Minute)
	return NewRouter(NewServer(svc), reg, metrics, logger)
}

func do(t *testing.T, e *echo.Echo, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestEndpointsSuccess(t *testing.T) {
	e := newTestRouter()
	tests := []struct {
		target string
		want   float64
	}{
		{"/api/sum?term_one=4&term_two=1", 5},
		{"/api/sub?term_one=4&term_two=1", 3},
		{"/api/mul?term_one=4&term_two=3", 12},
		{"/api/div?term_one=4&term_two=3", 1},               // truncado para inteiro
		{"/api/div?term_one=4&term_two=3&precision=2", 1.33}, // truncado a 2 casas
		{"/api/sum?term_one=1.5&term_two=2.5&precision=1", 4}, // 4.0
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			rec := do(t, e, tt.target)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var got CalculationResult
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal: %v (body=%s)", err, rec.Body.String())
			}
			if got.Result != tt.want {
				t.Errorf("result = %v, want %v", got.Result, tt.want)
			}
		})
	}
}

func TestIntegerResultHasNoDecimalPoint(t *testing.T) {
	e := newTestRouter()
	rec := do(t, e, "/api/sub?term_one=4&term_two=1")
	body := strings.TrimSpace(rec.Body.String())
	if body != `{"result":3}` {
		t.Errorf("body = %s, want {\"result\":3}", body)
	}
}

func TestDivisionByZeroReturns400(t *testing.T) {
	e := newTestRouter()
	rec := do(t, e, "/api/div?term_one=1&term_two=0")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var got Error
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Message == "" {
		t.Errorf("expected error message")
	}
}

func TestInvalidParamsReturn400(t *testing.T) {
	e := newTestRouter()
	cases := []string{
		"/api/sum?term_two=1",              // term_one ausente
		"/api/sum?term_one=1",              // term_two ausente
		"/api/sum?term_one=abc&term_two=1", // term_one não numérico
		"/api/sum?term_one=1&term_two=2&precision=99", // precision fora do range
		"/api/sum?term_one=1&term_two=2&precision=-1", // precision negativa
	}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			rec := do(t, e, target)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHealthEndpoints(t *testing.T) {
	e := newTestRouter()
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := do(t, e, path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", path, rec.Code)
		}
	}
}

func TestMetricsEndpoint(t *testing.T) {
	e := newTestRouter()
	// Gera tráfego para popular métricas.
	do(t, e, "/api/sum?term_one=1&term_two=1")
	rec := do(t, e, "/metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "calc_operations_total") {
		t.Errorf("expected calc_operations_total in metrics output")
	}
}
