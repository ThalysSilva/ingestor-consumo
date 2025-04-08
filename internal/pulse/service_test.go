package pulse

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/clients/mocks"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// mocks.MockRedisClient é um mock para a interface RedisClient
func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	originalMetrics := map[string]interface{}{
		"pulsesReceived":      pulsesReceived,
		"pulseProcessingTime": pulseProcessingTime,
		"redisAccessCount":    redisAccessCount,
		"channelBufferSize":   channelBufferSize,
		"pulsesProcessed":     pulsesProcessed,
	}

	pulsesReceived = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_received_total"})
	pulseProcessingTime = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ingestor_pulse_processing_duration_seconds"})
	redisAccessCount = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_redis_access_total"})

	channelBufferSize = prometheus.NewGauge(prometheus.GaugeOpts{Name: "ingestor_channel_buffer_size"})
	pulsesProcessed = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_processed_total"})

	exitCode := m.Run()

	pulsesReceived = originalMetrics["pulsesReceived"].(prometheus.Counter)
	pulseProcessingTime = originalMetrics["pulseProcessingTime"].(prometheus.Histogram)
	redisAccessCount = originalMetrics["redisAccessCount"].(prometheus.Counter)
	channelBufferSize = originalMetrics["channelBufferSize"].(prometheus.Gauge)
	pulsesProcessed = originalMetrics["pulsesProcessed"].(prometheus.Counter)

	os.Exit(exitCode)
}

func TestNewPulseService(t *testing.T) {
	t.Run("ValidParams", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)
		redisClient.On("Close").Return(nil)

		svc := NewPulseService(ctx, redisClient)
		assert.NotNil(t, svc)
	})

	t.Run("RedisClientError", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", fmt.Errorf("redis error"))
		redisClient.On("Close").Return(nil)

		svc := NewPulseService(ctx, redisClient)
		assert.Nil(t, svc)
	})

	t.Run("UsingCustomOptions", func(t *testing.T) {
		ctx := context.Background()
		initialRedisClient := new(mocks.MockRedisClient)
		initialRedisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		initialRedisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)
		initialRedisClient.On("Close").Return(nil)

		anotherRedisClient := new(mocks.MockRedisClient)
		var addressRedisClientSettled *clients.RedisClient

		svc := NewPulseService(ctx, initialRedisClient, func(ps *pulseService) {
			ps.redisClient = anotherRedisClient
			addressRedisClientSettled = &ps.redisClient
		})
		assert.NotNil(t, svc)
		assert.Equal(t, anotherRedisClient, *addressRedisClientSettled)
		assert.NotEqual(t, initialRedisClient, *addressRedisClientSettled)
	})

}

func TestStartAndStop(t *testing.T) {
	t.Run("ValidStartAndStop", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)

		redisClient.On("Get", ctx, "current_generation").Return("A", nil)
		testPulse := &Pulse{
			TenantId:   "tenant1",
			ProductSku: "sku1",
			UsedAmount: 100,
			UseUnit:    "KB",
		}
		redisClient.On("IncrByFloat", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB", testPulse.UsedAmount).Return(nil)

		svc := NewPulseService(ctx, redisClient)
		svc.EnqueuePulse(*testPulse)
		svc.Start(2, 100*time.Millisecond)
		time.Sleep(500 * time.Millisecond)
		redisClient.AssertExpectations(t)
		svc.Stop()
		assert.NotNil(t, svc)
	})

	t.Run("StartButErrorWithStorePulseInRedis", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("A", nil)
		testPulse := &Pulse{
			TenantId:   "tenant1",
			ProductSku: "sku1",
			UsedAmount: 100,
			UseUnit:    "KB",
		}
		redisClient.On("IncrByFloat", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB", testPulse.UsedAmount).Return(fmt.Errorf("redis error"))

		svc := NewPulseService(ctx, redisClient)
		svc.EnqueuePulse(*testPulse)
		svc.Start(2, 1*time.Minute)
		time.Sleep(time.Millisecond)
		redisClient.AssertExpectations(t)
		svc.Stop()
		assert.NotNil(t, svc)
	})
}

func TestEnqueuePulse(t *testing.T) {
	t.Run("ValidPulse", func(t *testing.T) {

		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("A", nil)
		redisClient.On("Close").Return(nil)
		svc := NewPulseService(ctx, redisClient)
		pulse := Pulse{
			TenantId:   "tenant1",
			ProductSku: "sku1",
			UsedAmount: 100,
			UseUnit:    "KB",
		}

		svc.EnqueuePulse(pulse)
		assert.True(t, true)
	})
	t.Run("EnqueueWithCancelledCtx", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		svc := &pulseService{
			ctx:       ctx,
			pulseChan: make(chan Pulse),
		}

		cancel()

		pulse := Pulse{
			TenantId:   "tenant1",
			ProductSku: "sku1",
			UsedAmount: 100.0,
			UseUnit:    KB,
		}

		svc.EnqueuePulse(pulse)

		select {
		case got := <-svc.pulseChan:
			t.Fatalf("Esperava que o canal estivesse vazio, mas recebeu: %+v", got)
		default:
		}
	})

}

func TestStorePulseInRedis(t *testing.T) {
	ctx := context.Background()
	redisClient := new(mocks.MockRedisClient)
	svc := &pulseService{
		redisClient: redisClient,
		ctx:         ctx,
	}
	svc.generationAtomic.Store("A")
	pulse := Pulse{
		TenantId:   "tenant1",
		ProductSku: "sku1",
		UsedAmount: 100,
		UseUnit:    "KB",
	}
	key := fmt.Sprintf("generation:A:tenant:%s:sku:%s:useUnit:%s", pulse.TenantId, pulse.ProductSku, pulse.UseUnit)
	redisClient.On("IncrByFloat", ctx, key, pulse.UsedAmount).Return(nil)

	err := svc.storePulseInRedis(ctx, redisClient, pulse)
	assert.NoError(t, err)
	redisClient.AssertExpectations(t)
}

func TestStorePulseInRedis_RetryOnError(t *testing.T) {
	ctx := context.Background()
	redisClient := new(mocks.MockRedisClient)
	svc := &pulseService{
		redisClient: redisClient,
		ctx:         ctx,
	}
	svc.generationAtomic.Store("A")
	pulse := Pulse{
		TenantId:   "tenant1",
		ProductSku: "sku1",
		UsedAmount: 100,
		UseUnit:    "KB",
	}
	key := fmt.Sprintf("generation:A:tenant:%s:sku:%s:useUnit:%s", pulse.TenantId, pulse.ProductSku, pulse.UseUnit)
	redisClient.On("IncrByFloat", ctx, key, pulse.UsedAmount).Return(fmt.Errorf("redis error")).Once()
	redisClient.On("IncrByFloat", ctx, key, pulse.UsedAmount).Return(nil).Once()

	err := svc.storePulseInRedis(ctx, redisClient, pulse)
	assert.NoError(t, err)
	redisClient.AssertExpectations(t)
}
