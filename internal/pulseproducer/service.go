package pulseproducer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type pulseProducerService struct {
	quitChan    chan struct{}
	wg          sync.WaitGroup
	minDelay    int
	maxDelay    int
	qtyTenants  int
	ingestorURL string
	skuMap      *map[string]pulse.PulseUnit
	qtySKUs     int
	httpClient  clients.HTTPClient
}
type PulseProducerService interface {
	// Inicia o serviço de produção de pulsos
	// Inicia a produção de pulsos com base na quantidade de Tenants e no intervalo de tempo
	Start()

	// Para o serviço de produção de pulsos
	Stop()
}

var qtyPulsesSent int64

// NewPulseProducerService cria um novo serviço de produção de pulsos.
// O serviço de produção de pulsos é responsável por gerar pulsos aleatórios
// Recebe a URL do ingestor, o intervalo mínimo e máximo de atraso entre os pulsos,
// a quantidade de Tenants e a quantidade de SKUs como parâmetros
// e retorna um ponteiro para o serviço de produção de pulsos.
// O cliente HTTP é configurado com um tempo limite de 5 segundos,
// 100 conexões simultâneas e um tempo limite de conexão ociosa de 30 segundos.
// É possível personalizar o cliente HTTP, se necessário.
// O serviço de produção de pulsos é iniciado com o método Start() e parado com o método Stop().
func NewPulseProducerService(ingestorURL string, minDelay, maxDelay, qtyTenants, qtySKUs int) PulseProducerService {
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:     30 * time.Second,
		},
	}
	psv := pulseProducerService{
		quitChan:    make(chan struct{}),
		minDelay:    minDelay,
		maxDelay:    maxDelay,
		qtyTenants:  qtyTenants,
		ingestorURL: ingestorURL,
		qtySKUs:     qtySKUs,
		httpClient:  httpClient,
	}

	psv.skuMap = psv.generateSkuMap()
	return &psv
}

var marshalFunc = json.Marshal
var uuidFunc = func() string { return uuid.New().String() }

func (s *pulseProducerService) Start() {
	for range s.qtyTenants {
		s.wg.Add(1)
		go func() {
			tenantId := uuidFunc()
			defer s.wg.Done()
			pulseCount := 1
			for {
				select {
				case <-s.quitChan:
					return
				default:
					delay := time.Duration(rand.Intn(s.maxDelay-s.minDelay)+s.minDelay) * time.Millisecond
					time.Sleep(delay)

					pulse, err := s.randomPulse(tenantId)
					if err != nil {
						log.Error().Msgf("Erro ao gerar pulso: %v\n", err)
						continue
					}
					log.Debug().Msgf("Gerando pulso: %v\n", pulse)
					pulseCount++
					atomic.AddInt64(&qtyPulsesSent, 1)

					jsonData, err := marshalFunc(*pulse)
					if err != nil {
						log.Error().Msgf("Erro ao codificar JSON: %v\n", err)
						continue
					}

					resp, err := s.httpClient.Post(s.ingestorURL, "application/json", bytes.NewBuffer(jsonData))
					if err != nil {
						log.Error().Msgf("Erro ao enviar pulso: %v\n", err)
						continue
					}
					resp.Body.Close()

					if resp.StatusCode != http.StatusNoContent {
						log.Error().Msgf("Erro ao enviar pulso. Status: %s\n", resp.Status)
					} else {
						log.Debug().Msgf("Pulso do cliente %s enviado com sucesso! Status: %s\n", tenantId, resp.Status)
					}
				}
			}
		}()
	}
}

func (s *pulseProducerService) Stop() {
	close(s.quitChan)
	s.wg.Wait()
	log.Info().Msgf("Total de pulsos enviados: %d \n", qtyPulsesSent)
}
// generateSkuMap gera um mapa de SKUs aleatórios com unidades de pulso associadas
func (s *pulseProducerService) generateSkuMap() *map[string]pulse.PulseUnit {
	skuMap := make(map[string]pulse.PulseUnit)
	for i := range s.qtySKUs {
		sku := fmt.Sprintf("SKU-%d", i)
		unit := s.randomPulseUnit()
		skuMap[sku] = unit
	}

	return &skuMap
}
