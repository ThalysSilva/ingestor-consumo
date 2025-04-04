package pulse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

type PulseService interface {
	EnqueuePulse(pulse Pulse)
	Start(workers int, intervalToSend time.Duration)
	Stop()
}

type pulseService struct {
	pulseChan    chan Pulse
	redisClient  *redis.Client
	ctx          context.Context
	wg           sync.WaitGroup
	generation   atomic.Value
	apiURLSender string
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
	pulsesSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_sent_total",
			Help: "Total de pulsos enviados para a API",
		},
	)
	pulsesFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_failed_total",
			Help: "Total de pulsos que falharam no envio para a API",
		},
	)
	aggregationCycleTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ingestor_aggregation_cycle_duration_seconds",
			Help:    "Duração do ciclo de agregação e envio",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(pulsesReceived, pulseProcessingTime, redisAccessCount, pulsesSent, pulsesFailed, aggregationCycleTime)
}

func NewPulseService(ctx context.Context, redisClient *redis.Client, apiURLSender string) PulseService {
	psv := &pulseService{
		pulseChan:    make(chan Pulse, 100),
		redisClient:  redisClient,
		ctx:          ctx,
		apiURLSender: apiURLSender,
	}
	psv.generation.Store("A")
	return psv
}

func (s *pulseService) Start(workers int, intervalToSend time.Duration) {
	for range workers {
		s.wg.Add(1)
		go s.processPulses()
	}
	s.startAggregationLoop(intervalToSend)
}

func (s *pulseService) Stop() {
	close(s.pulseChan)
	s.wg.Wait()
	fmt.Println("Todos os workers foram finalizados.")
}

func (s *pulseService) EnqueuePulse(pulse Pulse) {
	s.pulseChan <- pulse
}

func (s *pulseService) processPulses() {
	defer s.wg.Done()
	for pulse := range s.pulseChan {
		start := time.Now()
		if err := s.storePulseInRedis(s.ctx, s.redisClient, pulse); err != nil {
			fmt.Printf("Erro ao armazenar pulso no Redis: %v\n", err)
		} else {
			fmt.Printf("Pulso armazenado com sucesso: %s\n", pulse.TenantId)
			pulsesReceived.Inc()
		}
		pulseProcessingTime.Observe(time.Since(start).Seconds())
	}
}

func (s *pulseService) storePulseInRedis(ctx context.Context, client *redis.Client, pulse Pulse) error {
	gen := s.generation.Load().(string)
	key := fmt.Sprintf("generation:%s:tenant:%s:sku:%s:useUnit:%s", gen, pulse.TenantId, pulse.ProductSku, pulse.UseUnit)

	if err := client.IncrByFloat(ctx, key, pulse.UsedAmount).Err(); err != nil {
		return fmt.Errorf("erro ao incrementar valor no Redis: %v", err)
	}

	redisAccessCount.Inc()
	return nil
}

func (s *pulseService) processAndSendPulses() error {
	start := time.Now()
	defer func() {
		aggregationCycleTime.Observe(time.Since(start).Seconds())
	}()

	currentGen := s.generation.Load().(string)
	nextGen := "B"
	if currentGen == "B" {
		nextGen = "A"
	}

	pattern := fmt.Sprintf("generation:%s:tenant:*:sku:*:useUnit:*", currentGen)
	cursor := uint64(0)
	aggregatedPulses := make(map[string]Pulse)
	s.generation.Store(nextGen)
	fmt.Printf("Geração alternada para %s\n", nextGen)

	// Aguardar para não dar conflito com o worker que está processando os pulsos
	time.Sleep(5 * time.Second)

	for {
		batch, nextCursor, err := s.redisClient.Scan(s.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("erro ao escanear chaves no Redis: %v", err)
		}
		cursor = nextCursor

		for _, key := range batch {
			usedAmountStr, err := s.redisClient.Get(s.ctx, key).Result()
			if err != nil {
				fmt.Printf("Erro ao obter chave %s: %v\n", key, err)
				continue
			}

			usedAmount, err := strconv.ParseFloat(usedAmountStr, 64)
			if err != nil {
				fmt.Printf("Erro ao converter usedAmount para chave %s: %v\n", key, err)
				continue
			}

			var gen, tenantId, productSku, useUnitStr string
			_, err = fmt.Sscanf(key, "generation:%s:tenant:%s:sku:%s:useUnit:%s", &gen, &tenantId, &productSku, &useUnitStr)
			if err != nil {
				fmt.Printf("Erro ao parsear chave %s: %v\n", key, err)
				continue
			}

			pulse := Pulse{
				TenantId:   tenantId,
				ProductSku: productSku,
				UsedAmount: usedAmount,
				UseUnit:    PulseUnit(useUnitStr),
			}

			aggregatedPulses[key] = pulse
		}

		if cursor == 0 {
			break
		}
	}

	if len(aggregatedPulses) == 0 {
		fmt.Printf("Nenhum pulso para enviar na geração %s.\n", currentGen)
	} else {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		for key, pulse := range aggregatedPulses {
			jsonData, err := json.Marshal(pulse)
			if err != nil {
				fmt.Printf("Erro ao serializar pulso %s: %v\n", key, err)
				pulsesFailed.Inc()
				continue
			}

			resp, err := client.Post(s.apiURLSender, "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				fmt.Printf("Erro ao enviar pulso %s para a API: %v\n", key, err)
				pulsesFailed.Inc()
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("Erro na resposta da API para pulso %s: status %d\n", key, resp.StatusCode)
				pulsesFailed.Inc()
				continue
			}

			if err := s.redisClient.Del(s.ctx, key).Err(); err != nil {
				fmt.Printf("Erro ao apagar chave %s: %v\n", key, err)
				continue
			}

			pulsesSent.Inc()
			fmt.Printf("Pulso %s enviado e removido com sucesso\n", key)
		}
	}

	return nil
}

func (s *pulseService) startAggregationLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			fmt.Println("Processando e enviando pulsos agregados...")
			if err := s.processAndSendPulses(); err != nil {
				fmt.Printf("Erro ao processar e enviar pulsos: %v\n", err)
			} else {
				fmt.Println("Pulsos processados e enviados com sucesso.")
			}
		}
	}()
}

