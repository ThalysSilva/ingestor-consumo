package pulse

import "github.com/prometheus/client_golang/prometheus"

var (
	metricsRegistered = false
	pulsesReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_received_total",
			Help: "Total de pulsos recebidos pelo ingestor",
		},
	)
	pulseProcessingTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ingestor_pulse_processing_duration_seconds",
			Help:    "Duração do processamento dos pulsos",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
	)
	redisAccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_redis_access_total",
			Help: "Número total de acessos ao Redis",
		},
	)
	channelBufferSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ingestor_channel_buffer_size",
			Help: "Current number of pulses in the channel buffer",
		},
	)
	pulsesProcessed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_processed_total",
			Help: "Total de pulsos processados pelo ingestor",
		},
	)
)

func registerMetrics() {
	if metricsRegistered {
		return
	}
	metricsRegistered = true

	prometheus.MustRegister(
		pulsesReceived,
		pulseProcessingTime,
		redisAccessCount,
		channelBufferSize,
		pulsesProcessed,
	)
}