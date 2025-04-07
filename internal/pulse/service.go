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
	"github.com/rs/zerolog/log"
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
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
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
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
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

func init() {
	prometheus.MustRegister(pulsesReceived, pulseProcessingTime, redisAccessCount, pulsesBatchParsedFailed, aggregationCycleTime, pulsesSentFailed, pulsesSentSuccess, pulsesNotDeleted, channelBufferSize, pulsesProcessed)
}

func NewPulseService(ctx context.Context, redisClient clients.RedisClient, apiURLSender string, batchQtyToSend int, opts ...ServiceOptions) PulseService {
	if batchQtyToSend <= 0 {
		log.Error().Int("batch_qty", batchQtyToSend).Msg("batchQtyToSend deve ser maior que 0")
		panic(fmt.Sprintf("batchQtyToSend deve ser maior que 0, recebido: %d", batchQtyToSend))
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	psv := &pulseService{
		pulseChan:      make(chan Pulse, 50000),
		redisClient:    redisClient,
		httpClient:     httpClient,
		ctx:            ctx,
		apiURLSender:   apiURLSender,
		batchQtyToSend: batchQtyToSend,
	}
	currentGeneration, err := psv.getCurrentGeneration()
	if err != nil {
		log.Error().Err(err).Msg("Erro ao obter a geração atual")
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
	log.Info().Msg("Todos os workers foram finalizados")
}

func (s *pulseService) EnqueuePulse(pulse Pulse) {
	select {
	case s.pulseChan <- pulse:
		channelBufferSize.Set(float64(len(s.pulseChan)))
	case <-s.ctx.Done():
		return
	}
}

func (s *pulseService) processPulses() {
	defer s.wg.Done()
	for pulse := range s.pulseChan {
		start := time.Now()
		if err := s.storePulseInRedis(s.ctx, s.redisClient, pulse); err != nil {
			log.Error().Err(err).Str("tenant_id", pulse.TenantId).Msg("Erro ao armazenar pulso no Redis")
		} else {
			pulsesReceived.Inc()
		}
		duration := time.Since(start).Seconds()
		pulseProcessingTime.Observe(duration)
		channelBufferSize.Set(float64(len(s.pulseChan)))
		pulsesProcessed.Inc()
	}
}

func (s *pulseService) storePulseInRedis(ctx context.Context, client clients.RedisClient, pulse Pulse) error {
	return utils.Retry(func() error {
		gen := s.generation.Load().(string)
		key := fmt.Sprintf("generation:%s:tenant:%s:sku:%s:useUnit:%s", gen, pulse.TenantId, pulse.ProductSku, pulse.UseUnit)

		redisAccessCount.Inc()

		if err := client.IncrByFloat(ctx, key, pulse.UsedAmount).Err(); err != nil {
			log.Error().Str("key", key).Err(err).Msg("Erro ao armazenar pulso no Redis")
			return err
		}

		return nil
	}, 3)
}

var marshalFunc = json.Marshal

func (s *pulseService) sendPulses(stabilizationDelay time.Duration) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		aggregationCycleTime.Observe(duration)
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
				log.Error().Str("key", key).Err(err).Msg("Erro ao obter chave")
				continue
			}

			usedAmount, err := strconv.ParseFloat(usedAmountStr, 64)
			if err != nil {
				log.Error().Str("key", key).Err(err).Msg("Erro ao converter usedAmount para chave")
				continue
			}

			parts := strings.Split(key, ":")
			if len(parts) != 8 {
				log.Warn().Str("key", key).Msg("Chave inválida: formato esperado generation:<gen>:tenant:<tenantId>:sku:<productSku>:useUnit:<useUnit>")
				continue
			}

			gen := parts[1]
			tenantId := parts[3]
			productSku := parts[5]
			useUnitStr := parts[7]

			if gen == "" || tenantId == "" || productSku == "" || useUnitStr == "" {
				log.Warn().Str("key", key).Msg("Chave inválida: um ou mais campos estão vazios")
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
		log.Info().Str("generation", currentGen).Msg("Nenhum pulso para enviar")
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

			pulsesData, err := marshalFunc(pulses)
			if err != nil {
				log.Error().Int("batch_index", batchIndex).Str("generation", currentGen).Err(err).Msg("Erro ao serializar lote")
				pulsesBatchParsedFailed.Add(float64(len(pulses)))
				errChan <- fmt.Errorf("lote %d: erro ao serializar pulsos: %v", batchIndex, err)
				return
			}
			resp, err := s.httpClient.Post(s.apiURLSender, "application/json", bytes.NewBuffer(pulsesData))
			if err != nil {
				log.Error().Int("batch_index", batchIndex).Str("generation", currentGen).Err(err).Msg("Erro ao enviar lote para a API")
				pulsesSentFailed.Add(float64(len(pulses)))
				errChan <- fmt.Errorf("lote %d: erro ao enviar pulsos: %v", batchIndex, err)
				return
			}
			defer func() {
				if resp != nil {
					resp.Body.Close()
				}
			}()

			if resp.StatusCode != http.StatusOK {
				log.Error().Int("batch_index", batchIndex).Str("generation", currentGen).Int("status_code", resp.StatusCode).Msg("Erro na resposta da API")
				pulsesSentFailed.Add(float64(len(pulses)))
				errChan <- fmt.Errorf("lote %d: erro na resposta da API: status %d", batchIndex, resp.StatusCode)
				return
			}

			for _, pulse := range pulses {
				key := fmt.Sprintf("generation:%s:tenant:%s:sku:%s:useUnit:%s", currentGen, pulse.TenantId, pulse.ProductSku, pulse.UseUnit)
				if err := s.redisClient.Del(s.ctx, key).Err(); err != nil {
					log.Error().Str("key", key).Err(err).Msg("Erro ao apagar chave")
					pulsesNotDeleted.Inc()
					errChan <- fmt.Errorf("lote %d: erro ao apagar chave %s: %v", batchIndex, key, err)
					return
				}
			}

			pulsesSentSuccess.Add(float64(len(pulses)))
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
			log.Info().Msg("Processando e enviando pulsos agregados")
			if err := s.sendPulses(stabilizationDelay); err != nil {
				log.Error().Err(err).Msg("Erro ao processar e enviar pulsos")
			} else {
				log.Info().Msg("Pulsos processados e enviados com sucesso")
			}
		}
	}()
}
