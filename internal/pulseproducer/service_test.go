package pulseproducer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	ingestorURL = "http://localhost:8080/ingest"
)

type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	args := m.Called(url, contentType, body)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestNewPulseProducerService(t *testing.T) {

	pulseProducerService := NewPulseProducerService(ingestorURL, 500, 1000, 1, 1)

	if pulseProducerService == nil {
		t.Fatal("Failed to create PulseProducerService")
	}

}

func TestStartAndStop(t *testing.T) {
	t.Run("StartAndStopValid", func(t *testing.T) {
		mockHttpClient := new(MockHTTPClient)
		resp := &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
		mockHttpClient.On("Post", ingestorURL, "application/json", mock.AnythingOfType("*bytes.Buffer")).
			Return(resp, nil)

		pulseProducerService := &pulseProducerService{
			httpClient:  mockHttpClient,
			quitChan:    make(chan struct{}),
			minDelay:    1,
			maxDelay:    2,
			qtyTenants:  1,
			ingestorURL: ingestorURL,
			wg:          sync.WaitGroup{},
			skuMap: &map[string]pulse.PulseUnit{
				"SKU-0": pulse.KB,
			},
			qtySKUs: 1,
		}

		pulseProducerService.Start()
		time.Sleep(10 * time.Millisecond)
		mockHttpClient.AssertExpectations(t)

		pulseProducerService.Stop()

		select {
		case <-pulseProducerService.quitChan:
			// Canal fechado, Teste feito com sucesso
		default:
			t.Error("Expected quitChan to be closed, but it was not")
		}

	})

	t.Run("StartWithErroOnUUID", func(t *testing.T) {
		mockHttpClient := new(MockHTTPClient)
		originalUUIDFunc := uuidFunc
		defer func() {
			uuidFunc = originalUUIDFunc
		}()
		uuidFunc = func() string { return "" }

		svc := &pulseProducerService{
			wg:          sync.WaitGroup{},
			ingestorURL: ingestorURL,
			quitChan:    make(chan struct{}),
			skuMap:      &map[string]pulse.PulseUnit{"SKU-0": pulse.KB},
			qtyTenants:  1,
			qtySKUs:     1,
			minDelay:    1,
			maxDelay:    2,
			httpClient:  mockHttpClient,
		}

		svc.Start()
		time.Sleep(10 * time.Millisecond)
		mockHttpClient.AssertNotCalled(t, "Post", ingestorURL, "application/json", mock.AnythingOfType("*bytes.Buffer"))
		svc.Stop()

	})

	t.Run("StartWithErroOnMarshal", func(t *testing.T) {
		originalMarshalFunc := marshalFunc
		defer func() { marshalFunc = originalMarshalFunc }()
		marshalFunc = func(v any) ([]byte, error) {
			return nil, fmt.Errorf("falha intencional na serialização")
		}
		mockHttpClient := new(MockHTTPClient)
		svc := &pulseProducerService{
			wg:          sync.WaitGroup{},
			ingestorURL: ingestorURL,
			quitChan:    make(chan struct{}),
			skuMap:      &map[string]pulse.PulseUnit{"SKU-0": pulse.KB},
			qtyTenants:  1,
			qtySKUs:     1,
			minDelay:    1,
			maxDelay:    2,
			httpClient:  mockHttpClient,
		}

		svc.Start()
		time.Sleep(10 * time.Millisecond)
		mockHttpClient.AssertNotCalled(t, "Post", ingestorURL, "application/json", mock.AnythingOfType("*bytes.Buffer"))
		svc.Stop()

	})

	t.Run("StartWithStatusErrorOnPost", func(t *testing.T) {
		mockHttpClient := new(MockHTTPClient)
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
		mockHttpClient.On("Post", ingestorURL, "application/json", mock.AnythingOfType("*bytes.Buffer")).
			Return(resp, nil)

		svc := &pulseProducerService{
			wg:          sync.WaitGroup{},
			ingestorURL: ingestorURL,
			quitChan:    make(chan struct{}),
			skuMap:      &map[string]pulse.PulseUnit{"SKU-0": pulse.KB},
			qtyTenants:  1,
			qtySKUs:     1,
			minDelay:    1,
			maxDelay:    2,
			httpClient:  mockHttpClient,
		}

		svc.Start()
		time.Sleep(10 * time.Millisecond)
		mockHttpClient.AssertExpectations(t)
		svc.Stop()

	})

	t.Run("StartWithErrorOnPost", func(t *testing.T) {
		mockHttpClient := new(MockHTTPClient)
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}
		mockHttpClient.On("Post", ingestorURL, "application/json", mock.AnythingOfType("*bytes.Buffer")).
			Return(resp, fmt.Errorf("erro intencional"))

		svc := &pulseProducerService{
			wg:          sync.WaitGroup{},
			ingestorURL: ingestorURL,
			quitChan:    make(chan struct{}),
			skuMap:      &map[string]pulse.PulseUnit{"SKU-0": pulse.KB},
			qtyTenants:  1,
			qtySKUs:     1,
			minDelay:    1,
			maxDelay:    2,
			httpClient:  mockHttpClient,
		}

		svc.Start()
		time.Sleep(10 * time.Millisecond)
		mockHttpClient.AssertExpectations(t)
		svc.Stop()

	})
	t.Run("StopWithoutStart", func(t *testing.T) {
		pulseProducerService := &pulseProducerService{
			quitChan:    make(chan struct{}),
			minDelay:    1,
			maxDelay:    2,
			qtyTenants:  1,
			ingestorURL: ingestorURL,
			wg:          sync.WaitGroup{},
			skuMap: &map[string]pulse.PulseUnit{
				"SKU-0": pulse.KB,
			},
			qtySKUs: 1,
		}

		pulseProducerService.Stop()

		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				assert.True(t, ok, "O valor recuperado deve ser um erro")
				assert.Equal(t, "send on closed channel", err.Error(), "A mensagem de erro deve ser 'send on closed channel'")
			} else {
				t.Error("Esperava um panic ao enviar em um canal fechado, mas nenhum ocorreu")
			}
		}()
		pulseProducerService.quitChan <- struct{}{}
	})

}
