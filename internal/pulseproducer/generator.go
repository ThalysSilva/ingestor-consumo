package pulseproducer

import (
	"fmt"
	"math/rand"

	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/rs/zerolog/log"
)

func (pss *pulseProducerService) randomPulse(tenantId string) (*pulse.Pulse, error) {
	skuSelector := rand.Intn(len(*pss.skuMap))
	productSku := fmt.Sprintf("SKU-%d", skuSelector)
	useUnit := (*pss.skuMap)[productSku]
	usedAmount := float64(rand.Intn(1000)) + rand.Float64()
	pulse, err := pulse.NewPulse(tenantId, productSku, usedAmount, useUnit)
	if err != nil {
		log.Error().Msgf("Erro ao gerar pulso: %v", err)
		return nil, err
	}

	return pulse, nil
}

func (pss *pulseProducerService) randomPulseUnit() pulse.PulseUnit {
	units := []pulse.PulseUnit{pulse.KB, pulse.MB, pulse.GB, pulse.KBxSec, pulse.MBxSec, pulse.GBxSec}
	return units[rand.Intn(len(units))]
}
