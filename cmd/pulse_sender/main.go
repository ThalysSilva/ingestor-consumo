package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type pulseUnit string

const (
	KB     pulseUnit = "KB"
	MB     pulseUnit = "MB"
	GB     pulseUnit = "GB"
	KBxSec pulseUnit = "KB/sec"
	MBxSec pulseUnit = "MB/sec"
	GBxSec pulseUnit = "GB/sec"
)

type Pulse struct {
	TenantId   int       `json:"tenant_id"`
	ProductSku string    `json:"product_sku"`
	UsedAmount float64   `json:"used_amount"`
	UseUnit    pulseUnit `json:"use_unit"`
}

type pulseSender struct {
	quitChan chan struct{}
	wg       sync.WaitGroup
}

func (p pulseUnit) validatePulseUnit() bool {
	switch p {
	case KB, MB, GB, KBxSec, MBxSec, GBxSec:
		return true
	default:
		return false
	}
}

func NewPulseSender() *pulseSender {
	return &pulseSender{
		quitChan: make(chan struct{}),
	}
}

var (
	qtyPulsesSent int64
)

const (
	ingestorURL  = "http://localhost:8080/ingest"
	minDelay     = 500
	maxDelay     = 2000
	timeDuration = 10 * time.Second
	qtyTenants   = 10
)

func (s *pulseSender) Start() {

	for tenantId := range qtyTenants {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			pulseCount := 1
			for {
				select {
				case <-s.quitChan:
					return
				default:
					delay := time.Duration(rand.Intn(maxDelay-minDelay)+minDelay) * time.Millisecond
					time.Sleep(delay)
					pulse := GenerateRandomPulse()
					if !pulse.UseUnit.validatePulseUnit() {
						fmt.Printf("Unidade de pulso inválida: %s\n", pulse.UseUnit)
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
					resp, err := http.Post(ingestorURL, "application/json", bytes.NewBuffer(jsonData))
					if err != nil {
						fmt.Printf("Erro ao enviar pulso: %v\n", err)
						continue
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusNoContent {
						fmt.Printf("Erro ao enviar pulso. Status: %s\n", resp.Status)
					} else {
						fmt.Printf("Pulso do cliente %d enviado com sucesso! Status: %s\n", tenantId, resp.Status)
					}
				}
			}
		}()
	}
}

func (s *pulseSender) Stop(startTime time.Time) {
	close(s.quitChan)
	s.wg.Wait()
	timeDurationInSeconds := time.Since(startTime).Seconds()
	fmt.Printf("Total de pulsos enviados: %d Tempo decorrido: %ds", qtyPulsesSent, int(timeDurationInSeconds))
}

func generateRandomPulseUnit() pulseUnit {
	selector := rand.Intn(6)
	switch selector {
	case 0:
		return KB
	case 1:
		return MB
	case 2:
		return GB
	case 3:
		return KBxSec
	case 4:
		return MBxSec
	case 5:
		return GBxSec
	default:
		return KB
	}
}

func GenerateRandomPulse() *Pulse {
	tenantId := rand.Intn(qtyTenants) + 1
	productSku := fmt.Sprintf("SKU-%d", rand.Intn(1000))
	usedAmount := float64(rand.Intn(1000)) + rand.Float64()
	useUnit := generateRandomPulseUnit()
	return &Pulse{
		TenantId:   tenantId,
		ProductSku: productSku,
		UsedAmount: usedAmount,
		UseUnit:    useUnit,
	}
}

func main() {
	startTime := time.Now()
	rand.New(rand.NewSource(time.Now().UnixNano()))
	simulator := NewPulseSender()
	fmt.Println("Iniciando o Envio de pulsos...")

	simulator.Start()

	time.Sleep(10 * time.Second)
	simulator.Stop(startTime)

}
