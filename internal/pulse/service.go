package pulse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	pulseChan      chan Pulse
	redisClient    clients.RedisClient
	httpClient     clients.HTTPClient
	ctx            context.Context
	wg             sync.WaitGroup
	generation     atomic.Value
	apiURLSender   string
	batchQtyToSend int
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
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}, // Buckets para 1ms a 1s
		},
	)
	redisAccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ingestor_redis_access_total",
			Help: "Número total de acessos ao Redis",
		},
	)
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
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10}, // Buckets para 100ms a 10s
		},
	)
)

func init() {
	prometheus.MustRegister(pulsesReceived, pulseProcessingTime, redisAccessCount, pulsesBatchParsedFailed, aggregationCycleTime, pulsesSentFailed, pulsesSentSuccess, pulsesNotDeleted)
}

func NewPulseService(ctx context.Context, redisClient clients.RedisClient, apiURLSender string, batchQtyToSend int, opts ...ServiceOptions) PulseService {
	if batchQtyToSend <= 0 {
		panic(fmt.Sprintf("batchQtyToSend deve ser maior que 0, recebido: %d", batchQtyToSend))
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	psv := &pulseService{
		pulseChan:      make(chan Pulse, 100),
		redisClient:    redisClient,
		httpClient:     httpClient,
		ctx:            ctx,
		apiURLSender:   apiURLSender,
		batchQtyToSend: batchQtyToSend,
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
			fmt.Printf("Métrica pulsesReceived incrementada: %v\n", pulsesReceived)
		}
		duration := time.Since(start).Seconds()
		fmt.Printf("Duração do processamento do pulso: %f segundos\n", duration)
		pulseProcessingTime.Observe(duration)
		fmt.Printf("Métrica pulseProcessingTime observada: %f\n", duration)
	}
}

func (s *pulseService) storePulseInRedis(ctx context.Context, client clients.RedisClient, pulse Pulse) error {
	return utils.Retry(func() error {
		gen := s.generation.Load().(string)
		key := fmt.Sprintf("generation:%s:tenant:%s:sku:%s:useUnit:%s", gen, pulse.TenantId, pulse.ProductSku, pulse.UseUnit)

		redisAccessCount.Inc()
		fmt.Printf("Métrica redisAccessCount incrementada: %v\n", redisAccessCount)

		if err := client.IncrByFloat(ctx, key, pulse.UsedAmount).Err(); err != nil {
			return fmt.Errorf("erro ao incrementar valor no Redis: %v", err)
		}

		return nil
	}, 3)
}

func (s *pulseService) sendPulses(stabilizationDelay time.Duration) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		fmt.Printf("Duração do ciclo de agregação e envio: %f segundos\n", duration)
		aggregationCycleTime.Observe(duration)
		fmt.Printf("Métrica aggregationCycleTime observada: %f\n", duration)
	}()

	currentGen := s.generation.Load().(string)

	pattern := fmt.Sprintf("generation:%s:tenant:*:sku:*:useUnit:*", currentGen)
	cursor := uint64(0)
	aggregatedPulses := make(map[string]Pulse)

	if _, err := s.toggleGeneration(); err != nil {
		return fmt.Errorf("erro ao alternar geração: %v", err)
	}

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

			parts := strings.Split(key, ":")
			if len(parts) != 8 {
				fmt.Printf("Chave inválida %s: formato esperado generation:<gen>:tenant:<tenantId>:sku:<productSku>:useUnit:<useUnit>\n", key)
				continue
			}

			gen := parts[1]
			tenantId := parts[3]
			productSku := parts[5]
			useUnitStr := parts[7]

			if gen == "" || tenantId == "" || productSku == "" || useUnitStr == "" {
				fmt.Printf("Chave inválida %s: um ou mais campos estão vazios\n", key)
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
		return nil
	}

	pulsesBatch := utils.ChunkMapValues(aggregatedPulses, s.batchQtyToSend)

	const maxWorkers = 5
	semaphore := make(chan struct{}, maxWorkers)
	errChan := make(chan error, len(pulsesBatch))
	var wg sync.WaitGroup

	for batchIndex, pulses := range pulsesBatch {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(batchIndex int, pulses []Pulse) {
			defer wg.Done()
			defer func() { <-semaphore }()

			pulsesData, err := json.Marshal(pulses)
			if err != nil {
				fmt.Printf("Erro ao serializar lote %d da geração %s: %v\n", batchIndex, currentGen, err)
				pulsesBatchParsedFailed.Add(float64(len(pulses)))
				fmt.Printf("Métrica pulsesBatchParsedFailed incrementada: %v\n", pulsesBatchParsedFailed)
				errChan <- fmt.Errorf("lote %d: erro ao serializar pulsos: %v", batchIndex, err)
				return
			}

			resp, err := s.httpClient.Post(s.apiURLSender, "application/json", bytes.NewBuffer(pulsesData))
			if err != nil {
				fmt.Printf("Erro ao enviar lote %d da geração %s para a API: %v\n", batchIndex, currentGen, err)
				pulsesSentFailed.Add(float64(len(pulses)))
				fmt.Printf("Métrica pulsesSentFailed incrementada: %v\n", pulsesSentFailed)
				errChan <- fmt.Errorf("lote %d: erro ao enviar pulsos: %v", batchIndex, err)
				return
			}
			defer func() {
				if resp != nil {
					resp.Body.Close()
				}
			}()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("Erro na resposta da API para o lote %d da geração %s: status %d\n", batchIndex, currentGen, resp.StatusCode)
				pulsesSentFailed.Add(float64(len(pulses)))
				fmt.Printf("Métrica pulsesSentFailed incrementada: %v\n", pulsesSentFailed)
				errChan <- fmt.Errorf("lote %d: erro na resposta da API: status %d", batchIndex, resp.StatusCode)
				return
			}

			for _, pulse := range pulses {
				key := fmt.Sprintf("generation:%s:tenant:%s:sku:%s:useUnit:%s", currentGen, pulse.TenantId, pulse.ProductSku, pulse.UseUnit)
				if err := s.redisClient.Del(s.ctx, key).Err(); err != nil {
					fmt.Printf("Erro ao apagar chave %s: %v\n", key, err)
					pulsesNotDeleted.Inc()
					fmt.Printf("Métrica pulsesNotDeleted incrementada: %v\n", pulsesNotDeleted)
					errChan <- fmt.Errorf("lote %d: erro ao apagar chave %s: %v", batchIndex, key, err)
					return
				}
			}

			pulsesSentSuccess.Add(float64(len(pulses)))
			fmt.Printf("Métrica pulsesSentSuccess incrementada: %v\n", pulsesSentSuccess)
			fmt.Printf("Lote %d da geração %s enviado e removido com sucesso! (%d pulsos)\n", batchIndex, currentGen, len(pulses))
		}(batchIndex, pulses)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("falhas ao enviar pulsos: %v", errors)
	}

	fmt.Printf("Pulsos da geração %s enviados e removidos com sucesso!\n", currentGen)
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
			if err := s.sendPulses(stabilizationDelay); err != nil {
				fmt.Printf("Erro ao processar e enviar pulsos: %v\n", err)
			} else {
				fmt.Println("Pulsos processados e enviados com sucesso.")
			}
		}
	}()
}
