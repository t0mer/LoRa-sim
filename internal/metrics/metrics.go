// Package metrics defines Cylon's Prometheus collectors and the /metrics
// handler. Counters are incremented at the event/seam level; gauges are bound to
// live sources (running tags, WS clients) via GaugeFuncs.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Cylon's collectors registered on a private registry.
type Metrics struct {
	reg *prometheus.Registry

	Uplinks      *prometheus.CounterVec // labels: type
	Downlinks    *prometheus.CounterVec // labels: class
	Joins        *prometheus.CounterVec // labels: result
	JoinLatency  prometheus.Histogram
	RXWindowHits *prometheus.CounterVec // labels: window
	TxErrors     prometheus.Counter
	DBWrites     prometheus.Counter
	WSReconnects prometheus.Counter
}

// New creates and registers the collectors.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		Uplinks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cylon_uplinks_total", Help: "Uplinks sent by simulated tags.",
		}, []string{"type"}),
		Downlinks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cylon_downlinks_total", Help: "Downlinks delivered to tags.",
		}, []string{"class"}),
		Joins: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cylon_joins_total", Help: "OTAA join attempts by result.",
		}, []string{"result"}),
		JoinLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "cylon_join_latency_seconds", Help: "OTAA join latency.",
			Buckets: prometheus.DefBuckets,
		}),
		RXWindowHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cylon_rx_window_hits_total", Help: "Downlinks delivered per RX window.",
		}, []string{"window"}),
		TxErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cylon_tx_errors_total", Help: "Transmit errors.",
		}),
		DBWrites: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cylon_db_writes_total", Help: "Database write operations.",
		}),
		WSReconnects: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cylon_ws_reconnects_total", Help: "LNS WebSocket reconnects.",
		}),
	}
	reg.MustRegister(m.Uplinks, m.Downlinks, m.Joins, m.JoinLatency,
		m.RXWindowHits, m.TxErrors, m.DBWrites, m.WSReconnects)
	return m
}

// BindGauge registers a GaugeFunc reading live state from fn.
func (m *Metrics) BindGauge(name, help string, fn func() float64) {
	m.reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: name, Help: help,
	}, fn))
}

// Handler returns the /metrics HTTP handler.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// Registry exposes the underlying registry (for tests).
func (m *Metrics) Registry() *prometheus.Registry { return m.reg }
