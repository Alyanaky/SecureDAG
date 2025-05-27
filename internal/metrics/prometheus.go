package metrics

import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterMetrics() {
    prometheus.MustRegister(prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "securedag_operations_total",
            Help: "Total number of operations performed",
        },
    ))
    prometheus.MustRegister(prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "securedag_active_nodes",
            Help: "Number of active nodes in the network",
        },
    ))
}

func ExposeMetrics() {
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":9090", nil)
}
