package pulsesender

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
	"github.com/ThalysSilva/ingestor-consumo/internal/clients/mocks"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	originalMetrics := map[string]interface{}{

		"redisAccessCount":        redisAccessCount,
		"pulsesBatchParsedFailed": pulsesBatchParsedFailed,
		"pulsesSentFailed":        pulsesSentFailed,
		"pulsesSentSuccess":       pulsesSentSuccess,
		"pulsesNotDeleted":        pulsesNotDeleted,
		"aggregationCycleTime":    aggregationCycleTime,
	}

	redisAccessCount = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_redis_access_total"})
	pulsesBatchParsedFailed = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_batch_parse_failed_total"})
	pulsesSentFailed = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_sent_failed_total"})
	pulsesSentSuccess = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_sent_success_total"})
	pulsesNotDeleted = prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestor_pulses_not_deleted_total"})
	aggregationCycleTime = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ingestor_aggregation_cycle_duration_seconds"})

	originalMarshalFunc := marshalFunc
	defer func() { marshalFunc = originalMarshalFunc }()

	exitCode := m.Run()

	redisAccessCount = originalMetrics["redisAccessCount"].(prometheus.Counter)
	pulsesBatchParsedFailed = originalMetrics["pulsesBatchParsedFailed"].(prometheus.Counter)
	pulsesSentFailed = originalMetrics["pulsesSentFailed"].(prometheus.Counter)
	pulsesSentSuccess = originalMetrics["pulsesSentSuccess"].(prometheus.Counter)
	pulsesNotDeleted = originalMetrics["pulsesNotDeleted"].(prometheus.Counter)
	aggregationCycleTime = originalMetrics["aggregationCycleTime"].(prometheus.Histogram)

	os.Exit(exitCode)
}

func TestNewPulseSenderService(t *testing.T) {
	t.Run("ValidParams", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)
		redisClient.On("Close").Return(nil)

		svc := NewPulseSenderService(ctx, redisClient, "http://example.com", 10)
		assert.NotNil(t, svc)
	})
	t.Run("PanicOnInvalidBatchQty", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Close").Return(nil)

		assert.PanicsWithValue(t, "batchQtyToSend deve ser maior que 0, recebido: 0", func() {
			NewPulseSenderService(ctx, redisClient, "http://example.com", 0)
		})
	})

	t.Run("RedisClientError", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", fmt.Errorf("redis error"))
		redisClient.On("Close").Return(nil)

		svc := NewPulseSenderService(ctx, redisClient, "http://example.com", 10)
		assert.Nil(t, svc)
	})

	t.Run("UsingCustomOptions", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)
		redisClient.On("Close").Return(nil)

		httpClient := new(mocks.MockHTTPClient)
		var addressHttpClientSettled *clients.HTTPClient

		svc := NewPulseSenderService(ctx, redisClient, "http://example.com", 10, func(ps *pulseSenderService) {
			ps.httpClient = httpClient
			addressHttpClientSettled = &ps.httpClient
		})
		assert.NotNil(t, svc)
		assert.Equal(t, httpClient, *addressHttpClientSettled)
	})

	t.Run("UsingOptionWithCustomHTTPClient", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		redisClient.On("Get", ctx, "current_generation").Return("", redis.Nil)
		redisClient.On("Set", ctx, "current_generation", "A", time.Duration(0)).Return(nil)
		redisClient.On("Close").Return(nil)

		httpClient := new(mocks.MockHTTPClient)
		var addressHttpClientSettled *clients.HTTPClient
		svc := NewPulseSenderService(ctx, redisClient, "http://example.com", 10, WithCustomHTTPClient(
			func() clients.HTTPClient {
				return httpClient
			}(),
		), (func(ps *pulseSenderService) {
			addressHttpClientSettled = &ps.httpClient
		}))
		assert.NotNil(t, svc)
		assert.Equal(t, httpClient, *addressHttpClientSettled)

	})
}

func TestStartLoopAndStop(t *testing.T) {
	t.Run("ValidStartAndStop", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		clientHttp := new(mocks.MockHTTPClient)
		redisClient.On("Get", ctx, "current_generation").Return("A", nil)
		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(nil)

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
			"generation:A:tenant:tenant2:sku:sku2:useUnit:MB",
		}
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)).
			Return(keys, uint64(0), nil)
		clientHttp.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).Return(&http.Response{}, nil)

		svc := NewPulseSenderService(ctx, redisClient, "http://example.com", 10, WithCustomHTTPClient(clientHttp))
		svc.StartLoop(2, 100*time.Millisecond)
		time.Sleep(500 * time.Millisecond)
		redisClient.AssertExpectations(t)
		assert.NotNil(t, svc)
	})
}

func TestSendPulses(t *testing.T) {
	t.Run("ValidSendPulses", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

		keys := []string{
			"generation:A:tenant:tenant1:sku:sku1:useUnit:KB",
			"generation:A:tenant:tenant2:sku:sku2:useUnit:MB",
		}
		redisClient.On("Scan", ctx, uint64(0), "generation:A:tenant:*:sku:*:useUnit:*", int64(100)). // TO-CHECK: Dica para o uso de Scan
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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

		redisClient.On("Set", ctx, "current_generation", "B", time.Duration(0)).Return(fmt.Errorf("redis error"))

		err := svc.sendPulses(1 * time.Millisecond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis error")
		redisClient.AssertExpectations(t)
	})

	t.Run("ErrorInPostRequest", func(t *testing.T) {
		ctx := context.Background()
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
		redisClient := new(mocks.MockRedisClient)
		httpClient := new(mocks.MockHTTPClient)
		svc := &pulseSenderService{
			redisClient:    redisClient,
			httpClient:     httpClient,
			ctx:            ctx,
			apiURLSender:   "http://example.com",
			batchQtyToSend: 2,
		}

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
