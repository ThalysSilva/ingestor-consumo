package pulsesender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/generation"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/ThalysSilva/ingestor-consumo/pkg/utils"
	"github.com/rs/zerolog/log"
)

type pulseSenderService struct {
	ctx            context.Context
	redisClient    clients.RedisClient
	apiURLSender   string
	batchQtyToSend int
	generation     generation.ManagerGeneration
	httpClient     clients.HTTPClient
}

type PulseSenderService interface {
	// StartLoop inicia o loop de envio de pulsos.
	//
	// O interval define o intervalo entre os envios de pulsos.
	//
	// O stabilizationDelay define um tempo de espera após alternar a geração
	StartLoop(interval, stabilizationDelay time.Duration)
}

type ServiceOptions func(*pulseSenderService)

// WithCustomRedisClient permite definir um cliente Http personalizado
func WithCustomHTTPClient(client clients.HTTPClient) ServiceOptions {
	return func(ps *pulseSenderService) {
		ps.httpClient = client
	}
}

var marshalFunc = json.Marshal

func NewPulseSenderService(ctx context.Context, redisClient clients.RedisClient, apiURLSender string, batchQtyToSend int, opts ...ServiceOptions) PulseSenderService {
	if batchQtyToSend <= 0 {
		log.Error().Int("batch_qty", batchQtyToSend).Msg("batchQtyToSend deve ser maior que 0")
		panic(fmt.Sprintf("batchQtyToSend deve ser maior que 0, recebido: %d", batchQtyToSend))
	}

	registerMetrics()
	generation := generation.NewManagerGeneration(redisClient, ctx)
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	pss := &pulseSenderService{
		ctx:            ctx,
		redisClient:    redisClient,
		httpClient:     httpClient,
		apiURLSender:   apiURLSender,
		batchQtyToSend: batchQtyToSend,
		generation:     generation,
	}

	for _, opt := range opts {
		opt(pss)
	}

	return pss

}

func (s *pulseSenderService) StartLoop(interval, stabilizationDelay time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			log.Info().Msg("PulseSender: iniciando ciclo de envio")
			if err := s.sendPulses(stabilizationDelay); err != nil {
				log.Error().Err(err).Msg("Erro ao enviar pulsos")
			} else {
				log.Info().Msg("PulseSender: pulsos enviados com sucesso")
			}
		case <-s.ctx.Done():
			log.Info().Msg("PulseSender: finalizando loop de envio")
			ticker.Stop()
			return
		}
	}
}

func (s *pulseSenderService) sendPulses(stabilizationDelay time.Duration) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		aggregationCycleTime.Observe(duration)
	}()

	currentGen, err := s.generation.GetCurrentGeneration()
	if err != nil {
		log.Error().Err(err).Msg("Erro ao obter geração atual")
		return err
	}
	if _, err := s.generation.ToggleGeneration(); err != nil {
		return fmt.Errorf("erro ao alternar geração: %v", err)
	}
	time.Sleep(stabilizationDelay)

	pattern := fmt.Sprintf("generation:%s:tenant:*:sku:*:useUnit:*", currentGen)
	cursor := uint64(0)
	aggregatedPulses := make(map[string]pulse.Pulse)

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
				log.Error().Str("key", key).Err(err).Msg("Erro ao converter valor")
				continue
			}

			parts := strings.Split(key, ":")
			if len(parts) != 8 {
				log.Warn().Str("key", key).Msg("Chave inválida")
				continue
			}

			pulse := pulse.Pulse{
				TenantId:   parts[3],
				ProductSku: parts[5],
				UsedAmount: usedAmount,
				UseUnit:    pulse.PulseUnit(parts[7]),
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
		go func(batchIndex int, pulses []pulse.Pulse) {
			defer wg.Done()
			defer func() { <-semaphore }()

			pulsesData, err := marshalFunc(pulses)
			if err != nil {
				pulsesBatchParsedFailed.Add(float64(len(pulses)))
				errChan <- err
				return
			}
			resp, err := s.httpClient.Post(s.apiURLSender, "application/json", bytes.NewBuffer(pulsesData))
			if err != nil || resp.StatusCode != http.StatusOK {
				pulsesSentFailed.Add(float64(len(pulses)))
				errChan <- fmt.Errorf("falha envio lote %d: %v", batchIndex, err)
				return
			}
			resp.Body.Close()

			var keysToDelete []string
			for _, pulse := range pulses {
				key := fmt.Sprintf("generation:%s:tenant:%s:sku:%s:useUnit:%s",
					currentGen, pulse.TenantId, pulse.ProductSku, pulse.UseUnit)
				keysToDelete = append(keysToDelete, key)
			}

			if err := s.redisClient.Del(s.ctx, keysToDelete...).Err(); err != nil {
				for range pulses {
					pulsesNotDeleted.Inc()
				}
				errChan <- fmt.Errorf("erro ao apagar chaves do lote %d: %v", batchIndex, err)
				return
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
		return fmt.Errorf("falhas no envio: %v", errors)
	}
	return nil
}
