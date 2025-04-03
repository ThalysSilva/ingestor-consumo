package pulse

import (
	"github.com/ThalysSilva/ingestor-consumo/internal/entities"
	"math/rand"
)

type IngestorService interface {
	processPulses(pulses []entities.Pulse) error
}

func RandomPulseUnit() entities.PulseUnit {
	units := []entities.PulseUnit{entities.PulseUnitKB, entities.PulseUnitMB, entities.PulseUnitGB, entities.PulseUnitKBxSec, entities.PulseUnitMBxSec, entities.PulseUnitGBxSec}
	return units[rand.Intn(len(units))]
}
