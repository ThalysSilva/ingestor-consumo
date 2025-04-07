package pulse

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRedisClient é um mock para a interface RedisClient
type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd {
	args := m.Called(ctx, key, value)
	cmd := redis.NewFloatCmd(ctx, args...)
	if err := args.Error(0); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal(value)
	}
	return cmd
}

func (m *MockRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	args := m.Called(ctx, cursor, match, count)
	scanCmd := redis.NewScanCmd(ctx, nil, "SCAN", cursor, "MATCH", match, "COUNT", count)
	if keys, ok := args.Get(0).([]string); ok {
		// Obtém o próximo cursor como uint64 do segundo argumento (índice 1)
		var nextCursor uint64
		if cursorVal, ok := args.Get(1).(uint64); ok {
			nextCursor = cursorVal
		}
		scanCmd.SetVal(keys, nextCursor) // Define as chaves retornadas e o próximo cursor
	}
	if err := args.Error(2); err != nil {
		scanCmd.SetErr(err)
	}
	return scanCmd
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	cmd := redis.NewStringCmd(ctx, args...)
	if val, ok := args.Get(0).(string); ok {
		cmd.SetVal(val)
	}
	if err := args.Error(1); err != nil {
		cmd.SetErr(err)
	}
	return cmd
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	cmd := redis.NewStatusCmd(ctx, args...)
	if err := args.Error(0); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal("OK")
	}
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	cmd := redis.NewIntCmd(ctx, args...)
	if err := args.Error(0); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal(int64(len(keys)))
	}
	return cmd
}

func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	cmd := redis.NewStatusCmd(ctx, args...)
	if err := args.Error(0); err != nil {
		cmd.SetErr(err)
	} else {
		cmd.SetVal("PONG")
	}
	return cmd
}

func (m *MockRedisClient) PoolStats() *redis.PoolStats {
	args := m.Called()
	return args.Get(0).(*redis.PoolStats)
}

// MockHTTPClient é um mock para a interface HTTPClient
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	args := m.Called(url, contentType, body)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	originalMetrics := map[string]interface{}{
		"pulsesReceived":          pulsesReceived,
		"pulseProcessingTime":     pulseProcessingTime,
		"redisAccessCount":        redisAccessCount,
		"pulsesBatchParsedFailed": pulsesBatchParsedFailed,
		"pulsesSentFailed":        pulsesSentFailed,
		"pulsesSentSuccess":       pulsesSentSuccess,
		"pulsesNotDeleted":        pulsesNotDeleted,
		"aggregationCycleTime":    aggregationCycleTime,
		"channelBufferSize":       channelBufferSize,
		"pulsesProcessed":         pulsesProcessed,
	}

	pulsesReceived = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_received_total"})
	pulseProcessingTime = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ingestor_pulse_processing_duration_seconds"})
	redisAccessCount = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_redis_access_total"})
	pulsesBatchParsedFailed = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_batch_parse_failed_total"})
	pulsesSentFailed = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_sent_failed_total"})
	pulsesSentSuccess = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_sent_success_total"})
	pulsesNotDeleted = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_not_deleted_total"})
	aggregationCycleTime = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ingestor_aggregation_cycle_duration_seconds"})
	channelBufferSize = prometheus.NewGauge(prometheus.GaugeOpts{Name: "ingestor_channel_buffer_size"})
	pulsesProcessed = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_processed_total"})

	originalMarshalFunc := marshalFunc
	defer func() { marshalFunc = originalMarshalFunc }()

	exitCode := m.Run()

	pulsesReceived = originalMetrics["pulsesReceived"].(prometheus.Counter)
	pulseProcessingTime = originalMetrics["pulseProcessingTime"].(prometheus.Histogram)
	redisAccessCount = originalMetrics["redisAccessCount"].(prometheus.Counter)
	pulsesBatchParsedFailed = originalMetrics["pulsesBatchParsedFailed"].(prometheus.Counter)
	pulsesSentFailed = originalMetrics["pulsesSentFailed"].(prometheus.Counter)
	pulsesSentSuccess = originalMetrics["pulsesSentSuccess"].(prometheus.Counter)
	pulsesNotDeleted = originalMetrics["pulsesNotDeleted"].(prometheus.Counter)
	aggregationCycleTime = originalMetrics["aggregationCycleTime"].(prometheus.Histogram)
	channelBufferSize = originalMetrics["channelBufferSize"].(prometheus.Gauge)
	pulsesProcessed = originalMetrics["pulsesProcessed"].(prometheus.Counter)

	os.Exit(exitCode)
}

func TestNewPulseService(t *testing.T) {
	t.Run("ValidParams", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)
		redisClient.On("Close").Return(nil)

		svc := NewPulseService(ctx, redisClient, "http://example.com", 10)
		assert.NotNil(t, svc)
	})
	t.Run("PanicOnInvalidBatchQty", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		redisClient.On("Close").Return(nil)

		assert.PanicsWithValue(t, "batchQtyToSend deve ser maior que 0, recebido: 0", func() {
			NewPulseService(ctx, redisClient, "http://example.com", 0)
		})
	})

	t.Run("RedisClientError", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", fmt.Errorf("redis error"))
		redisClient.On("Close").Return(nil)

		svc := NewPulseService(ctx, redisClient, "http://example.com", 10)
		assert.Nil(t, svc)
	})

	t.Run("UsingCustomOptions", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)
		redisClient.On("Close").Return(nil)

		httpClient := new(MockHTTPClient)
		var addressHttpClientSettled *clients.HTTPClient

		svc := NewPulseService(ctx, redisClient, "http://example.com", 10, func(ps *pulseService) {
			ps.httpClient = httpClient
			addressHttpClientSettled = &ps.httpClient
		})
		assert.NotNil(t, svc)
		assert.Equal(t, httpClient, *addressHttpClientSettled)
	})

	t.Run("UsingOptionWithCustomHTTPClient", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)
		redisClient.On("Close").Return(nil)

		httpClient := new(MockHTTPClient)
		var addressHttpClientSettled *clients.HTTPClient
		svc := NewPulseService(ctx, redisClient, "http://example.com", 10, WithCustomHTTPClient(
			func() clients.HTTPClient {
				return httpClient
			}(),
		), (func(ps *pulseService) {
			addressHttpClientSettled = &ps.httpClient
		}))
		assert.NotNil(t, svc)
		assert.Equal(t, httpClient, *addressHttpClientSettled)

	})
}

func TestStartAndStop(t *testing.T) {
	t.Run("ValidStartAndStop", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		clientHttp := new(MockHTTPClient)
		redisClient.On("Get", ctx, "current_generation").Return("A", nil)
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		testPulse := &Pulse{
			TenantId:   "tenant1",
			ProductSku: "sku1",
			UsedAmount: 100,
			UseUnit:    "KB",
		}
		redisClient.On("IncrByFloat", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB", testPulse.UsedAmount).Return(nil)
		clientHttp.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).Return(&http.Response{}, nil)

		svc := NewPulseService(ctx, redisClient, "http://example.com", 10, WithCustomHTTPClient(clientHttp))
		svc.EnqueuePulse(*testPulse)
		svc.Start(2, 100*time.Millisecond)
		time.Sleep(500 * time.Millisecond)
		redisClient.AssertExpectations(t)
		svc.Stop()
		assert.NotNil(t, svc)
	})

	t.Run("StartButErrorWithStorePulseInRedis", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		clientHttp := new(MockHTTPClient)
		redisClient.On("Get", ctx, "current_generation").Return("A", nil)
		testPulse := &Pulse{
			TenantId:   "tenant1",
			ProductSku: "sku1",
			UsedAmount: 100,
			UseUnit:    "KB",
		}
		clientHttp.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).Return(&http.Response{}, nil)
		redisClient.On("IncrByFloat", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB", testPulse.UsedAmount).Return(fmt.Errorf("redis error"))

		svc := NewPulseService(ctx, redisClient, "http://example.com", 10, WithCustomHTTPClient(clientHttp))
		svc.EnqueuePulse(*testPulse)
		svc.Start(2, 1*time.Minute)
		time.Sleep(time.Millisecond)
		redisClient.AssertExpectations(t)
		svc.Stop()
		assert.NotNil(t, svc)
	})
}

func TestProcessPulses(t *testing.T) {

}

func TestEnqueuePulse(t *testing.T) {
	t.Run("ValidPulse", func(t *testing.T) {

		ctx := context.Background()
		redisClient := new(MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("A", nil)
		redisClient.On("Close").Return(nil)
		svc := NewPulseService(ctx, redisClient, "http://example.com", 10)
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
	redisClient := new(MockRedisClient)
	svc := &pulseService{
		redisClient: redisClient,
		ctx:         ctx,
	}
	svc.generation.Store("A")
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
	redisClient := new(MockRedisClient)
	svc := &pulseService{
		redisClient: redisClient,
		ctx:         ctx,
	}
	svc.generation.Store("A")
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

func TestGetCurrentGeneration(t *testing.T) {
	t.Run("GenerationExists", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		svc := &pulseService{
			redisClient: redisClient,
			ctx:         ctx,
		}
		redisClient.On("Get", ctx, "current_generation").Return("A", nil)

		gen, err := svc.getCurrentGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", gen)
		redisClient.AssertExpectations(t)
	})

	t.Run("GenerationNotExists", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		svc := &pulseService{
			redisClient: redisClient,
			ctx:         ctx,
		}
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)

		gen, err := svc.getCurrentGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", gen)
		redisClient.AssertExpectations(t)
	})

	t.Run("RedisErrorOnCreateGeneration", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		svc := &pulseService{
			redisClient: redisClient,
			ctx:         ctx,
		}
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(fmt.Errorf("redis error"))

		_, err := svc.getCurrentGeneration()
		assert.Error(t, err)
		redisClient.AssertExpectations(t)
	})

	t.Run("WrongGeneration", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		svc := &pulseService{
			redisClient: redisClient,
			ctx:         ctx,
		}
		redisClient.On("Get", ctx, "current_generation").Return("B", nil)

		gen, err := svc.getCurrentGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "B", gen)
		redisClient.AssertExpectations(t)
	})
}

func TestToggleGeneration(t *testing.T) {
	t.Run("FromAToB", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		svc := &pulseService{
			redisClient: redisClient,
			ctx:         ctx,
		}
		svc.generation.Store("A")
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)

		nextGen, err := svc.toggleGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "B", nextGen)
		assert.Equal(t, "B", svc.generation.Load().(string))
		redisClient.AssertExpectations(t)
	})

	t.Run("FromBToA", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		svc := &pulseService{
			redisClient: redisClient,
			ctx:         ctx,
		}
		svc.generation.Store("B")
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)

		nextGen, err := svc.toggleGeneration()
		assert.NoError(t, err)
		assert.Equal(t, "A", nextGen)
		assert.Equal(t, "A", svc.generation.Load().(string))
		redisClient.AssertExpectations(t)
	})
}

func TestSendPulses(t *testing.T) {
	t.Run("ValidSendPulses", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
			"generation:A:tenant:tenant2:sku:sku2:useUnit:MB",
		}
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB").
			Return("100", nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant2:sku:sku2:useUnit:MB").
			Return("200", nil)
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
		httpClient.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).
			Return(resp, nil)
		redisClient.On("Del", ctx, []string{"generation:A:tenant:tenant1:sku:sku1:useUnit:KB"}).Return(nil)
		redisClient.On("Del", ctx, []string{"generation:A:tenant:tenant2:sku:sku2:useUnit:MB"}).Return(nil)

		err := svc.sendPulses(1 * time.Millisecond)
		assert.NoError(t, err)
		redisClient.AssertExpectations(t)
		httpClient.AssertExpectations(t)

	})
	t.Run("ErrorInGenerationToggle", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(fmt.Errorf("redis error"))

		err := svc.sendPulses(1 * time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis error")
		redisClient.AssertExpectations(t)
	})

	t.Run("ErrorInPostRequest", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB").
			Return("100", nil)
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)

		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
		httpClient.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).
			Return(resp, nil)

		err := svc.sendPulses(1 * time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "erro na resposta da API: status 500")
		redisClient.AssertExpectations(t)
		httpClient.AssertExpectations(t)
	})

	t.Run("ErrorOnScanRedis", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(nil, uint64(0), fmt.Errorf("scan error"))

		err := svc.sendPulses(1 * time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "scan error")
		redisClient.AssertExpectations(t)
	})
	t.Run("ErrorOnGetRedis", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB").
			Return("", fmt.Errorf("get error"))
		httpClient.AssertNotCalled(t, "Post", mock.Anything, mock.Anything, mock.Anything)
		_ = svc.sendPulses(1 * time.Millisecond)
		redisClient.AssertExpectations(t)
	})

	t.Run("ErrorOnStringifyUsedAmount", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB").
			Return("invalid", nil)

		_ = svc.sendPulses(1 * time.Millisecond)
		httpClient.AssertNotCalled(t, "Post", mock.Anything, mock.Anything, mock.Anything)
		redisClient.AssertExpectations(t)
	})
	t.Run("ErrorOnSplitKey", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "tenant:tenant1:sku:sku1:useUnit:KB").
			Return("100", nil)

		_ = svc.sendPulses(1 * time.Millisecond)
		httpClient.AssertNotCalled(t, "Post", mock.Anything, mock.Anything, mock.Anything)
		redisClient.AssertExpectations(t)
	})

	t.Run("ErrorOnValidateSplitedKey", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation::tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation::tenant:tenant1:sku:sku1:useUnit:KB").
			Return("100", nil)

		_ = svc.sendPulses(1 * time.Millisecond)
		httpClient.AssertNotCalled(t, "Post", mock.Anything, mock.Anything, mock.Anything)
		redisClient.AssertExpectations(t)
	})

	t.Run("ErrorOnMarshallBatchPulses", func(t *testing.T) {
		originalMarshalFunc := marshalFunc
		defer func() { marshalFunc = originalMarshalFunc }()
		marshalFunc = func(v any) ([]byte, error) {
			return nil, fmt.Errorf("falha intencional na serialização")
		}

		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB").
			Return("100", nil)

		err := svc.sendPulses(1 * time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "falhas ao enviar pulsos")
		assert.Contains(t, err.Error(), "lote 0: erro ao serializar pulsos: falha intencional na serialização")

		httpClient.AssertNotCalled(t, "Post", mock.Anything, mock.Anything, mock.Anything)
		httpClient.AssertExpectations(t)
	})

	t.Run("ErrorOnPostRequestWithBatchPulseNotEqualStatusOk", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB").
			Return("100", nil)

		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
		httpClient.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).
			Return(resp, nil)

		err := svc.sendPulses(1 * time.Millisecond)
		assert.Contains(t, err.Error(), "erro na resposta da API")
		assert.Contains(t, err.Error(), "status 500")

		httpClient.AssertExpectations(t)
	})

	t.Run("PostRequestWithBatchPulseReturningError", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB").
			Return("100", nil)

		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}

		httpClient.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).
			Return(resp, fmt.Errorf("erro ao enviar pulsos"))

		err := svc.sendPulses(1 * time.Millisecond)
		assert.Contains(t, err.Error(), "erro ao enviar pulsos")

		httpClient.AssertExpectations(t)
	})

	t.Run("ErrorOnDeleteKeys", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(MockRedisClient)
		httpClient := new(MockHTTPClient)
		svc := &pulseService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}
		svc.generation.Store("A")

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
		}
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		redisClient.On("Get", ctx, "generation:A:tenant:tenant1:sku:sku1:useUnit:KB").
			Return("100", nil)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
		httpClient.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).
			Return(resp, nil)

		redisClient.On("Del", ctx, []string{"generation:A:tenant:tenant1:sku:sku1:useUnit:KB"}).Return(fmt.Errorf("falha ao excluir chaves no Redis"))
		err := svc.sendPulses(1 * time.Millisecond)
		httpClient.AssertExpectations(t)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "falha ao excluir chaves no Redis")
	})

}
