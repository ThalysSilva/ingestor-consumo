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

	t.Run("Key exists", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return(redis.NewStringResult("A", nil)).Once()
		gen, err := mgr.GetCurrentGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", gen)
	})

	t.Run("Key not found (redis.Nil)", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return(redis.NewStringResult("", redis.Nil)).Once()
		mockRedis.On("Set", ctx, "current_generation", "A", mock.Anything).Return(redis.NewStatusResult("OK", nil)).Once()
		gen, err := mgr.GetCurrentGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", gen)
	})

	t.Run("Redis error", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return(redis.NewStringResult("", errors.New("boom"))).Once()
		_, err := mgr.GetCurrentGeneration()
		assert.Error(t, err)
	})
}

func TestToggleGeneration(t *testing.T) {
	ctx := context.Background()
	mockRedis := new(mocks.MockRedisClient)
	mgr := &managerGeneration{redisClient: mockRedis, ctx: ctx}

	t.Run("Toggle A to B", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return(redis.NewStringResult("A", nil)).Once()
		mockRedis.On("Set", ctx, "current_generation", "B", mock.Anything).Return(redis.NewStatusResult("OK", nil)).Once()
		next, err := mgr.ToggleGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "B", next)
	})

	t.Run("Toggle B to A", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return(redis.NewStringResult("B", nil)).Once()
		mockRedis.On("Set", ctx, "current_generation", "A", mock.Anything).Return(redis.NewStatusResult("OK", nil)).Once()
		next, err := mgr.ToggleGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", next)
	})

	t.Run("Set fails", func(t *testing.T) {
		mockRedis.On("Get", ctx, "current_generation").Return(redis.NewStringResult("B", nil)).Once()
		mockRedis.On("Set", ctx, "current_generation", "A", mock.Anything).Return(redis.NewStatusResult("", errors.New("fail"))).Once()
		_, err := mgr.ToggleGeneration()
		assert.Error(t, err)
	})
}