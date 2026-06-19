package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: parseLevel("info")})
	slog.New(h).Info("hello", "k", "v")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}
	if entry["msg"] != "hello" || entry["k"] != "v" {
		t.Errorf("unexpected log entry: %v", entry)
	}
}

func TestNewLoggerFormats(t *testing.T) {
	if NewLogger("debug", "json") == nil {
		t.Error("json logger is nil")
	}
	if NewLogger("info", "text") == nil {
		t.Error("text logger is nil")
	}
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"bogus":   slog.LevelInfo,
	}
	for in, want := range tests {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNewMetricsRegistersCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.Operations.WithLabelValues("sum").Inc()
	m.CacheHits.WithLabelValues("sum").Inc()
	m.CacheErrors.Inc()
	// Métricas *Vec só aparecem no Gather após terem ao menos uma observação.
	m.HTTPRequests.WithLabelValues("GET", "/api/sum", "200").Inc()
	m.HTTPDuration.WithLabelValues("GET", "/api/sum").Observe(0.1)

	if got := testutil.ToFloat64(m.Operations.WithLabelValues("sum")); got != 1 {
		t.Errorf("operations = %v, want 1", got)
	}

	// As métricas devem estar registradas no registry fornecido.
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var names []string
	for _, f := range families {
		names = append(names, f.GetName())
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{"calc_operations_total", "calc_cache_hits_total", "calc_http_requests_total"} {
		if !strings.Contains(joined, want) {
			t.Errorf("metric %q not registered (have: %s)", want, joined)
		}
	}
}
