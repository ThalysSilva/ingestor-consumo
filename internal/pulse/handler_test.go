package pulse

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPulseService é um mock para a interface PulseService
type MockPulseService struct {
	mock.Mock
}
func (m *MockPulseService) EnqueuePulse(pulse Pulse) {
	m.Called(pulse)
}

func (m *MockPulseService) Start(workers int, intervalToSend time.Duration)  {
	m.Called(workers, intervalToSend)
}
func (m *MockPulseService) Stop() {
	m.Called()
}

func TestPulseHandler_Ingestor_Success(t *testing.T) {
	// Configura o modo de teste do Gin
	gin.SetMode(gin.TestMode)

	// Cria um mock para o PulseService
	pulseService := new(MockPulseService)
	handler := NewPulseHandler(pulseService)

	// Define um Pulse válido para o teste
	validPulse := Pulse{
		TenantId:   "tenant1",
		ProductSku: "sku1",
		UsedAmount: 100.0,
		UseUnit:    KB,
	}
	pulseService.On("EnqueuePulse", validPulse).Return()

	// Cria uma requisição HTTP POST com o Pulse válido
	body, _ := json.Marshal(validPulse)
	req, _ := http.NewRequest("POST", "/ingestor", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Cria um contexto Gin e chama o handler
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	handler.Ingestor()(c)

	// Verifica a resposta
	assert.Equal(t, http.StatusNoContent, w.Code) // 204 No Content
	assert.Empty(t, w.Body.String())              // Corpo vazio
	pulseService.AssertExpectations(t)            // Verifica que EnqueuePulse foi chamado
}

func TestPulseHandler_Ingestor_InvalidJSON(t *testing.T) {
	// Configura o modo de teste do Gin
	gin.SetMode(gin.TestMode)

	// Cria um mock para o PulseService
	pulseService := new(MockPulseService)
	handler := NewPulseHandler(pulseService)

	// Cria uma requisição HTTP POST com JSON inválido
	req, _ := http.NewRequest("POST", "/ingestor", bytes.NewBuffer([]byte(`{"invalid":`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Cria um contexto Gin e chama o handler
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	handler.Ingestor()(c)

	// Verifica a resposta
	assert.Equal(t, http.StatusBadRequest, w.Code) // 400 Bad Request
	expectedBody := `{"error":"Invalid request"}`
	assert.JSONEq(t, expectedBody, w.Body.String())
	pulseService.AssertNotCalled(t, "EnqueuePulse", mock.Anything) // Verifica que EnqueuePulse não foi chamado
}

func TestPulseHandler_Ingestor_MissingFields(t *testing.T) {
	// Configura o modo de teste do Gin
	gin.SetMode(gin.TestMode)

	// Cria um mock para o PulseService
	pulseService := new(MockPulseService)
	handler := NewPulseHandler(pulseService)

	// Define um Pulse com campos obrigatórios ausentes
	invalidPulse := map[string]interface{}{
		"tenant_id": "tenant1",
		// Falta product_sku, used_amount e use_unit
	}
	body, _ := json.Marshal(invalidPulse)
	req, _ := http.NewRequest("POST", "/ingestor", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Cria um contexto Gin e chama o handler
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	handler.Ingestor()(c)

	// Verifica a resposta
	assert.Equal(t, http.StatusBadRequest, w.Code) // 400 Bad Request
	expectedBody := `{"error":"Invalid request"}`
	assert.JSONEq(t, expectedBody, w.Body.String())
	pulseService.AssertNotCalled(t, "EnqueuePulse", mock.Anything) // Verifica que EnqueuePulse não foi chamado
}

func TestPulseHandler_Ingestor_InvalidUseUnit(t *testing.T) {
	// Configura o modo de teste do Gin
	gin.SetMode(gin.TestMode)

	// Cria um mock para o PulseService
	pulseService := new(MockPulseService)
	handler := NewPulseHandler(pulseService)

	// Define um Pulse com use_unit inválido
	invalidPulse := Pulse{
		TenantId:   "tenant1",
		ProductSku: "sku1",
		UsedAmount: 100.0,
		UseUnit:    "INVALID", // Unidade inválida
	}
	body, _ := json.Marshal(invalidPulse)
	req, _ := http.NewRequest("POST", "/ingestor", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Cria um contexto Gin e chama o handler
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	handler.Ingestor()(c)
	

	// Verifica a resposta
	assert.Equal(t, http.StatusBadRequest, w.Code) 
	expectedBody := `{"error": "Invalid pulse unit"}`
	assert.JSONEq(t, expectedBody, w.Body.String())
	pulseService.AssertNotCalled(t, "EnqueuePulse", mock.Anything)
}
