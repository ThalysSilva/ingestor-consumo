package pulsesender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)


var qtyPulsesSent int64

func NewPulseSender(ingestorURL string, minDelay, maxDelay, qtyTenants int) *PulseSender {
	return &PulseSender{
		quitChan:    make(chan struct{}),
		minDelay:    minDelay,
		maxDelay:    maxDelay,
		qtyTenants:  qtyTenants,
		ingestorURL: ingestorURL,
	}
}

func (p *PulseSender) Start() {
	for range p.qtyTenants {
		p.wg.Add(1)
		go func() {
			tenantId := uuid.New().String()
			defer p.wg.Done()
			pulseCount := 1
			for {
				select {
				case <-p.quitChan:
					return
				default:
					delay := time.Duration(rand.Intn(p.maxDelay-p.minDelay)+p.minDelay) * time.Millisecond
					time.Sleep(delay)

					pulse, err := GenerateRandomPulse(tenantId)
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

					resp, err := http.Post(p.ingestorURL, "application/json", bytes.NewBuffer(jsonData))
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

func (s *PulseSender) Stop() {
	close(s.quitChan)
	s.wg.Wait()

	fmt.Printf("Total de pulsos enviados: %d \n", qtyPulsesSent)
}
