package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsService struct {
	reg prometheus.Gatherer
}

func NewMetricsService(reg prometheus.Gatherer) *MetricsService {
	return &MetricsService{reg}
}

func (m *MetricsService) Mux(mux *http.ServeMux) {
	mux.Handle("GET /metrics", promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}))
}
