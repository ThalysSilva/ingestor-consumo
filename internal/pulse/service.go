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

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/pkg/utils"
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
	redisClient  clients.RedisClient
	httpClient   clients.HTTPClient
	ctx          context.Context
	wg           sync.WaitGroup
	generation   atomic.Value
	apiURLSender string
}

type ServiceOptions func(*pulseService)

func WithCustomHTTPClient(client clients.HTTPClient) ServiceOptions {
	return func(ps *pulseService) {
		ps.httpClient = client
	}
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
	pulsesFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_failed_total",
			Help: "Total de pulsos que falharam no envio para a API",
		},
	)
	pulsesSentFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_sent_failed_total",
			Help: "Total de envio de pulsos falhados",
		},
	)
	pulsesSentSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_pulses_sent_success_total",
			Help: "Total de envio de pulsos com sucesso",
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
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(pulsesReceived, pulseProcessingTime, redisAccessCount, pulsesFailed, aggregationCycleTime, pulsesSentFailed, pulsesSentSuccess, pulsesNotDeleted)
}

func NewPulseService(ctx context.Context, redisClient clients.RedisClient, apiURLSender string, opts ...ServiceOptions) PulseService {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	psv := &pulseService{
		pulseChan:    make(chan Pulse, 100),
		redisClient:  redisClient,
		httpClient:   httpClient,
		ctx:          ctx,
		apiURLSender: apiURLSender,
	}
	currentGeneration, err := psv.getCurrentGeneration()
	if err != nil {
		fmt.Printf("Erro ao obter a geração atual: %v\n", err)
		return nil
	}
	psv.generation.Store(currentGeneration)

	for _, opt := range opts {
		opt(psv)
	}

	return psv
}

func (s *pulseService) Start(workers int, intervalToSend time.Duration) {
	for range workers {
		s.wg.Add(1)
		go s.processPulses()
	}
	s.startAggregationLoop(intervalToSend, 5*time.Second)
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

func (s *pulseService) storePulseInRedis(ctx context.Context, client clients.RedisClient, pulse Pulse) error {
	return utils.Retry(func() error {
		gen := s.generation.Load().(string)
		key := fmt.Sprintf("generation:%s:tenant:%s:sku:%s:useUnit:%s", gen, pulse.TenantId, pulse.ProductSku, pulse.UseUnit)

		if err := client.IncrByFloat(ctx, key, pulse.UsedAmount).Err(); err != nil {
			return fmt.Errorf("erro ao incrementar valor no Redis: %v", err)
		}

		redisAccessCount.Inc()
		return nil
	}, 3)
}

func (s *pulseService) processAndSendPulses(stabilizationDelay time.Duration) error {
	start := time.Now()
	defer func() {
		aggregationCycleTime.Observe(time.Since(start).Seconds())
	}()

	currentGen := s.generation.Load().(string)

	pattern := fmt.Sprintf("generation:%s:tenant:*:sku:*:useUnit:*", currentGen)
	cursor := uint64(0)
	aggregatedPulses := make(map[string]Pulse)

	if _, err := s.toggleGeneration(); err != nil {
		return fmt.Errorf("erro ao alternar geração: %v", err)
	}

	// Aguardar para não dar conflito com o worker que está processando os pulsos
	time.Sleep(stabilizationDelay)

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

		if cursor == 0 { // entender melhor
			break
		}
	}

	if len(aggregatedPulses) == 0 {
		fmt.Printf("Nenhum pulso para enviar na geração %s.\n", currentGen)
		return nil
	}

	pulses := make([]Pulse, 0, len(aggregatedPulses))
	for _, value := range aggregatedPulses {
		pulses = append(pulses, value)
	}

	pulsesData, err := json.Marshal(pulses)
	if err != nil {
		fmt.Printf("Erro ao serializar pulsos: %v\n", err)
		pulsesSentFailed.Inc()
		pulsesFailed.Inc()
		return err
	}

	resp, err := s.httpClient.Post(s.apiURLSender, "application/json", bytes.NewBuffer(pulsesData))
	if err != nil {
		fmt.Printf("Erro ao enviar pulsos da geração \"%s\" para a API: %v\n", currentGen, err)
		pulsesSentFailed.Inc()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Erro na resposta da API para os pulsos da geração \"%s\". status %d\n", currentGen, resp.StatusCode)
		pulsesSentFailed.Inc()
		return err
	}

	for key := range aggregatedPulses {
		if err := s.redisClient.Del(s.ctx, key).Err(); err != nil {
			err = fmt.Errorf("erro ao apagar chave %s: %v", key, err)
			// pulsesNotDeleted.Inc()
			panic(err) // melhorar essa tratativa
		}
	}

	pulsesSentSuccess.Inc()
	fmt.Printf("Pulsos da geração \"%s\" enviados e removidos com sucesso!", currentGen)

	return nil
}

func (s *pulseService) getCurrentGeneration() (string, error) {
	gen, err := s.redisClient.Get(s.ctx, "current_generation").Result()
	if err == redis.Nil {
		if err := s.redisClient.Set(s.ctx, "current_generation", "A", 0).Err(); err != nil {
			return "", err
		}
		return "A", nil
	} else if err != nil {
		return "", err
	}
	return gen, nil
}

func (s *pulseService) toggleGeneration() (nextGen string, err error) {
	currentGen := s.generation.Load().(string)

	nextGen = "B"
	if currentGen == "B" {
		nextGen = "A"
	}

	if err := s.redisClient.Set(s.ctx, "current_generation", nextGen, 0).Err(); err != nil {
		return "", err
	}
	s.generation.Store(nextGen)
	return nextGen, nil
}

func (s *pulseService) startAggregationLoop(interval time.Duration, stabilizationDelay time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			fmt.Println("Processando e enviando pulsos agregados...")
			if err := s.processAndSendPulses(stabilizationDelay); err != nil {
				fmt.Printf("Erro ao processar e enviar pulsos: %v\n", err)
			} else {
				fmt.Println("Pulsos processados e enviados com sucesso.")
			}
		}
	}()
}
