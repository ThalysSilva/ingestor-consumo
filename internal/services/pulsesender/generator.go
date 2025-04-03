package pulsesender

import (
	"fmt"
	"math/rand"

	"github.com/ThalysSilva/ingestor-consumo/internal/entities"
	"github.com/ThalysSilva/ingestor-consumo/internal/services/pulse"
)

func GenerateRandomPulse(tenantId string) (*entities.Pulse, error) {
	productSku := fmt.Sprintf("SKU-%d", rand.Intn(1000))
	usedAmount := float64(rand.Intn(1000)) + rand.Float64()
	useUnit := pulse.RandomPulseUnit()
	pulse, err := entities.NewPulse(tenantId, productSku, usedAmount, useUnit)
	if err != nil {
		fmt.Printf("Erro ao gerar pulso: %v\n", err)
		return nil, err
	}

	return pulse, nil
}
