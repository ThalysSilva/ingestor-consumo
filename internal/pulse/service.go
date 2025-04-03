package pulse

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

type pulseService struct {
	pulsoChannel chan Pulse
	redisClient  *redis.Client
	ctx          context.Context
	wg           sync.WaitGroup
}

type PulseService interface {
	EnqueuePulse(pulso Pulse)
	Start(workers int)
	Stop()
}

var (
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
			Buckets: prometheus.DefBuckets,
		},
	)
	redisAccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_redis_access_total",
			Help: "Número total de acessos ao Redis",
		},
	)
)

func init() {
	prometheus.MustRegister(pulsesReceived, pulseProcessingTime, redisAccessCount)
}

func NewPulseService(ctx context.Context, redisClient *redis.Client) PulseService {
	return &pulseService{
		pulsoChannel: make(chan Pulse, 100),
		redisClient:  redisClient,
		ctx:          ctx,
	}
}

func (s *pulseService) Start(workers int) {
	for range workers {
		s.wg.Add(1)
		go s.processPulses()
	}
}

func (s *pulseService) Stop() {
	close(s.pulsoChannel)
	s.wg.Wait()
	fmt.Println("Todos os workers foram finalizados.")
}

func (s *pulseService) EnqueuePulse(pulse Pulse) {
	s.pulsoChannel <- pulse
}

func (s *pulseService) processPulses() {
	defer s.wg.Done()
	for pulso := range s.pulsoChannel {
		start := time.Now()
		if err := storePulseInRedis(s.ctx, s.redisClient, pulso); err != nil {
			fmt.Printf("Erro ao armazenar pulso no Redis: %v\n", err)
		} else {
			fmt.Printf("Pulso armazenado com sucesso: %s\n", pulso.TenantId)
			pulsesReceived.Inc()
		}
		pulseProcessingTime.Observe(time.Since(start).Seconds())
	}

}

func storePulseInRedis(ctx context.Context, client *redis.Client, pulse Pulse) error {
	data, err := json.Marshal(pulse)
	if err != nil {
		return err
	}
	redisAccessCount.Inc()
	return client.Set(ctx, "tenantId:"+pulse.TenantId, data, 10*time.Minute).Err()
}

func RandomPulseUnit() PulseUnit {
	units := []PulseUnit{KB, MB, GB, KBxSec, MBxSec, GBxSec}
	return units[rand.Intn(len(units))]
}
