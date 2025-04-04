package pulsesender

import (
	"fmt"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"math/rand"
)

func GenerateRandomPulse(tenantId string, useUnit pulse.PulseUnit) (*pulse.Pulse, error) {
	productSku := fmt.Sprintf("SKU-%d", rand.Intn(1000))
	usedAmount := float64(rand.Intn(1000)) + rand.Float64()
	pulse, err := pulse.NewPulse(tenantId, productSku, usedAmount, useUnit)
	if err != nil {
		fmt.Printf("Erro ao gerar pulso: %v\n", err)
		return nil, err
	}

	return pulse, nil
}
