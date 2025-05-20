package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	StorageUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "storage_usage_bytes",
			Help: "Total storage usage in bytes",
		},
	)
)

func Init() {
	prometheus.MustRegister(RequestsTotal, StorageUsage)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
