package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandlerExposesCollectors(t *testing.T) {
	m := New()
	m.Uplinks.WithLabelValues("data").Inc()
	m.Joins.WithLabelValues("success").Inc()
	m.BindGauge("cylon_active_tags", "Running tags.", func() float64 { return 3 })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	m.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{
		`cylon_uplinks_total{type="data"} 1`,
		`cylon_joins_total{result="success"} 1`,
		`cylon_active_tags 3`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}
