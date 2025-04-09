package pulsesender

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsRegistered = false
	pulsesBatchParsedFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_batch_parse_failed_total",
			Help: "Total de pulsos que falharam ao serem serializados para envio à API",
		},
	)
	
	pulsesSentFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_sent_failed_total",
			Help: "Total de pulsos que falharam ao serem enviados para a API",
		},
	)
	pulsesSentSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_sent_success_total",
			Help: "Total de pulsos enviados com sucesso para a API",
		},
	)
	pulsesNotDeleted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_not_deleted_total",
			Help: "Total de pulsos que não foram deletados do Redis",
		},
	)
	aggregationCycleTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ingestor_aggregation_cycle_duration_seconds",
			Help:    "Duração do ciclo de agregação e envio",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
	)
)

func registerMetrics() {
	if metricsRegistered {
		return
	}
	metricsRegistered = true

	prometheus.MustRegister(
		pulsesSentFailed,
		pulsesSentSuccess,
		pulsesNotDeleted,
		aggregationCycleTime,
	)
}
