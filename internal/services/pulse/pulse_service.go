package pulse

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/entities"
	"github.com/go-redis/redis/v8"
)

type pulseService struct {
	pulsoChannel chan entities.Pulse
	redisClient  *redis.Client
	ctx          context.Context
	wg           sync.WaitGroup
}

type PulseService interface {
	ProcessPulses()
	StorePulseInRedis(ctx context.Context, client *redis.Client, pulso entities.Pulse) error
}



func NewPulseService(ctx context.Context) *pulseService {
	return &pulseService{
		pulsoChannel: make(chan entities.Pulse, 100),
	}
}

func (s *pulseService) ProcessPulsos() {
	for pulso := range s.pulsoChannel {

		if err := storePulseInRedis(s.ctx, s.redisClient, pulso); err != nil {
			fmt.Printf("Erro ao armazenar pulso no Redis: %v\n", err)
		} else {
			fmt.Printf("Pulso armazenado com sucesso: %s\n", pulso.TenantId)

		}

	}
	s.wg.Done()
}

func RandomPulseUnit() entities.PulseUnit {
	units := []entities.PulseUnit{entities.PulseUnitKB, entities.PulseUnitMB, entities.PulseUnitGB, entities.PulseUnitKBxSec, entities.PulseUnitMBxSec, entities.PulseUnitGBxSec}
	return units[rand.Intn(len(units))]
}

func storePulseInRedis(ctx context.Context, client *redis.Client, pulse entities.Pulse) error {
	data, err := json.Marshal(pulse)
	if err != nil {
		return err
	}

	return client.Set(ctx, "pulso:"+pulse.TenantId, data, 10*time.Minute).Err()
}
