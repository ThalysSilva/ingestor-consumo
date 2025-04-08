package generation

import (
	"context"
	"testing"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients/mocks"
	"github.com/stretchr/testify/assert"
)

func TestNewManagerGeneration(t *testing.T) {
	mockRedis := new(mocks.MockRedisClient)
	ctx := context.Background()

	mgr := NewManagerGeneration(mockRedis, ctx)
	assert.NotNil(t, mgr)
}
