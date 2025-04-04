package pulsesender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/google/uuid"
)

type pulseSenderService struct {
	quitChan    chan struct{}
	wg          sync.WaitGroup
	minDelay    int
	maxDelay    int
	qtyTenants  int
	ingestorURL string
}
type PulseSenderService interface {
	Start()
	Stop()
}

var qtyPulsesSent int64

func NewPulseSenderService(ingestorURL string, minDelay, maxDelay, qtyTenants int) PulseSenderService {
	return &pulseSenderService{
		quitChan:    make(chan struct{}),
		minDelay:    minDelay,
		maxDelay:    maxDelay,
		qtyTenants:  qtyTenants,
		ingestorURL: ingestorURL,
	}
}

func (ps *pulseSenderService) Start() {
	for range ps.qtyTenants {
		ps.wg.Add(1)
		go func() {
			tenantId := uuid.New().String()
			useUnit := pulse.RandomPulseUnit()
			defer ps.wg.Done()
			pulseCount := 1
			for {
				select {
				case <-ps.quitChan:
					return
				default:
					delay := time.Duration(rand.Intn(ps.maxDelay-ps.minDelay)+ps.minDelay) * time.Millisecond
					time.Sleep(delay)

					pulse, err := GenerateRandomPulse(tenantId, useUnit)
					if err != nil {
						fmt.Printf("Erro ao gerar pulso: %v\n", err)
						continue
					}

					fmt.Printf("Enviando pulso N°%d: %+v (delay: %v)\n", pulseCount, pulse, delay)
					pulseCount++
					atomic.AddInt64(&qtyPulsesSent, 1)

					jsonData, err := json.Marshal(*pulse)
					if err != nil {
						fmt.Printf("Erro ao codificar JSON: %v\n", err)
						continue
					}

					resp, err := http.Post(ps.ingestorURL, "application/json", bytes.NewBuffer(jsonData))
					if err != nil {
						fmt.Printf("Erro ao enviar pulso: %v\n", err)
						continue
					}
					resp.Body.Close()

					if resp.StatusCode != http.StatusNoContent {
						fmt.Printf("Erro ao enviar pulso. Status: %s\n", resp.Status)
					} else {
						fmt.Printf("Pulso do cliente %s enviado com sucesso! Status: %s\n", tenantId, resp.Status)
					}
				}
			}
		}()
	}
}

func (ps *pulseSenderService) Stop() {
	close(ps.quitChan)
	ps.wg.Wait()

	fmt.Printf("Total de pulsos enviados: %d \n", qtyPulsesSent)
}
