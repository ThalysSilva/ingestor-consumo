package generation

import (
	"context"
	"errors"
	"testing"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients/mocks"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetCurrentGeneration(t *testing.T) {
	ctx := context.Background()
	mockRedis := new(mocks.MockRedisClient)
	mgr := &managerGeneration{redisClient: mockRedis, ctx: ctx}

	t.Run("KeyExists", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return("A", nil).Once()
		gen, err := mgr.GetCurrentGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", gen)
	})

	t.Run("KeyNotFound", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return("", redis.Nil).Once()
		mockRedis.On("Set", ctx, "current_generation", "A", mock.Anything).Return(nil).Once()
		gen, err := mgr.GetCurrentGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", gen)
	})

	t.Run("RedisError", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return("", errors.New("boom")).Once()
		_, err := mgr.GetCurrentGeneration()
		assert.Error(t, err)
	})
}

func TestToggleGeneration(t *testing.T) {
	ctx := context.Background()
	mockRedis := new(mocks.MockRedisClient)
	mgr := &managerGeneration{redisClient: mockRedis, ctx: ctx}

	t.Run("ToggleA-B", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return("A", nil).Once()
		mockRedis.On("Set", ctx, "current_generation", "B", mock.Anything).Return(nil).Once()
		next, err := mgr.ToggleGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "B", next)
	})

	t.Run("ToggleB-A", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return("B", nil).Once()
		mockRedis.On("Set", ctx, "current_generation", "A", mock.Anything).Return(nil).Once()
		next, err := mgr.ToggleGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", next)
	})

	t.Run("SetFails", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return("B", nil).Once()
		mockRedis.On("Set", ctx, "current_generation", "A", mock.Anything).Return(errors.New("fail")).Once()
		_, err := mgr.ToggleGeneration()
		assert.Error(t, err)
	})
}