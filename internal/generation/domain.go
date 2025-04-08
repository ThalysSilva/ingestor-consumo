package generation

import (
	"context"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
)

type managerGeneration struct {
	redisClient clients.RedisClient
	ctx         context.Context
}

type ManagerGeneration interface {
	ToggleGeneration() (string, error)
	GetCurrentGeneration() (string, error)
}

func NewManagerGeneration(redisClient clients.RedisClient, ctx context.Context) ManagerGeneration {
	return &managerGeneration{
		redisClient: redisClient,
		ctx:         ctx,
	}
}
