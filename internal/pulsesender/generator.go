package pulsesender

import (
	"fmt"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"math/rand"
)

func (pss *pulseSenderService) randomPulse(tenantId string) (*pulse.Pulse, error) {
	skuSelector := rand.Intn(len(*pss.skuMap))
	productSku := fmt.Sprintf("SKU-%d", skuSelector)
	useUnit := (*pss.skuMap)[productSku]
	usedAmount := float64(rand.Intn(1000)) + rand.Float64()
	pulse, err := pulse.NewPulse(tenantId, productSku, usedAmount, useUnit)
	if err != nil {
		fmt.Printf("Erro ao gerar pulso: %v\n", err)
		return nil, err
	}

	return pulse, nil
}

func (pss *pulseSenderService) randomPulseUnit() pulse.PulseUnit {
	units := []pulse.PulseUnit{pulse.KB, pulse.MB, pulse.GB, pulse.KBxSec, pulse.MBxSec, pulse.GBxSec}
	return units[rand.Intn(len(units))]
}
