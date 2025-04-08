package clients

import (
	"errors"
	"testing"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients/mocks"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

func TestRedisInit(t *testing.T) {
	t.Run("InitWithValidPing", func(t *testing.T) {
		zerolog.SetGlobalLevel(zerolog.Disabled)
		redisClientMock := new(mocks.MockRedisClient)
		redisClientMock.On("Ping", mock.Anything).Return(nil)
		redisClientMock.On("PoolStats").Return(&redis.PoolStats{
			Hits:     1,
			Misses:   0,
			Timeouts: 0,
		})
		client := InitRedisClient("localhost", "6379", nil, WithCustomRedisClient(redisClientMock))

		if client == nil {
			t.Fatal("Failed to create RedisClient")
		}
		redisClientMock.AssertExpectations(t)
	})
	t.Run("InitWithInvalidPing", func(t *testing.T) {
		zerolog.SetGlobalLevel(zerolog.Disabled)
		redisClientMock := new(mocks.MockRedisClient)
		defer (func() {
			if r := recover(); r == nil {
				t.Fatal("Expected panic but did not occur")
			}
		})()

		redisClientMock.On("Ping", mock.Anything).Return(errors.New("ping error"))
		_ = InitRedisClient("localhost", "6379", nil, WithCustomRedisClient(redisClientMock))

		redisClientMock.AssertExpectations(t)
	})
}
