package pulse

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/generation"
	"github.com/ThalysSilva/ingestor-consumo/pkg/utils"
	"github.com/rs/zerolog/log"
)

type PulseService interface {
	// EnqueuePulse adiciona um pulso ao canal pulseChan para processamento.
	// O método verifica se o contexto foi cancelado antes de adicionar o pulso ao canal.
	// Se o canal estiver cheio, o método não adiciona o pulso e não bloqueia.
	EnqueuePulse(pulse Pulse)

	// Start inicia o serviço de pulsos, criando os workers para processar os pulsos recebidos.
	// O parâmetro workers define o número de workers a serem criados.
	// O parâmetro refreshTimeGeneration define o intervalo de tempo entre as verificações de geração atual.
	// O método aguarda a finalização de todos os workers antes de retornar.
	Start(workers int, refreshTimeGeneration time.Duration)

	// Stop finaliza o serviço de pulsos, fechando o canal de pulsos e aguardando a finalização dos workers.
	// O método aguarda a finalização de todos os workers antes de retornar.
	// O método não deve ser chamado antes de iniciar o serviço.
	Stop()
}

type pulseService struct {
	pulseChan        chan Pulse
	redisClient      clients.RedisClient
	ctx              context.Context
	wg               sync.WaitGroup
	generationAtomic atomic.Value
	generation       generation.ManagerGeneration
}

type ServiceOptions func(*pulseService)

// NewPulseService cria uma nova instância do serviço de pulsos
// com um cliente Redis e uma URL de API para envio de pulsos
// O parâmetro batchQtyToSend define a quantidade de pulsos a serem enviados em cada lote
// O parâmetro opts permite passar opções adicionais para o serviço
// O parâmetro ctx é o contexto de execução
// O parâmetro redisClient é o cliente Redis a ser utilizado
// O parâmetro apiURLSender é a URL da API para envio de pulsos após agregação
func NewPulseService(ctx context.Context, redisClient clients.RedisClient, opts ...ServiceOptions) PulseService {
	registerMetrics()
	generation := generation.NewManagerGeneration(redisClient, ctx)

	psv := &pulseService{
		pulseChan:   make(chan Pulse, 50000),
		redisClient: redisClient,
		ctx:         ctx,
		generation:  generation,
	}
	currentGeneration, err := generation.GetCurrentGeneration()
	if err != nil {
		log.Error().Err(err).Msg("Erro ao obter a geração atual")
		return nil
	}
	psv.generationAtomic.Store(currentGeneration)

	for _, opt := range opts {
		opt(psv)
	}

	return psv
}

func (s *pulseService) refreshCurrentGeneration(timeout time.Duration) {
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentGen, err := s.generation.GetCurrentGeneration()
			if err != nil {
				log.Error().Err(err).Msg("Erro ao obter geração atual")
				continue
			}
			s.generationAtomic.Store(currentGen)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *pulseService) Start(workers int, refreshTimeGeneration time.Duration) {
	go s.refreshCurrentGeneration(refreshTimeGeneration)
	for range workers {
		s.wg.Add(1)
		go s.processPulses()
	}
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

// processPulses processa os pulsos recebidos do canal pulseChan
// O método aguarda a chegada de pulsos e os armazena no Redis
// Caso ocorra um erro ao armazenar o pulso, ele é registrado no log
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

// O método é executado em um goroutine e aguarda a finalização do worker
// O método processa os pulsos recebidos do canal pulseChan e os armazena no Redis
// Caso ocorra um erro ao armazenar o pulso, ele é registrado no log
func (s *pulseService) storePulseInRedis(ctx context.Context, client clients.RedisClient, pulse Pulse) error {
	return utils.Retry(func() error {
		gen := s.generationAtomic.Load().(string)
		key := fmt.Sprintf("generation:%s:tenant:%s:sku:%s:useUnit:%s", gen, pulse.TenantId, pulse.ProductSku, pulse.UseUnit)

		redisAccessCount.Inc()

		if err := client.IncrByFloat(ctx, key, pulse.UsedAmount).Err(); err != nil {
			log.Error().Str("key", key).Err(err).Msg("Erro ao armazenar pulso no Redis")
			return err
		}

		return nil
	}, 3)
}
