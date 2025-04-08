package clients

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients/mocks"
	"github.com/stretchr/testify/mock"
)

func TestHTTPClient(t *testing.T) {
	httpClient := new(mocks.MockHTTPClient)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}
	httpClient.On("Post", "http://example.com", "application/json", mock.AnythingOfType("*bytes.Buffer")).
		Return(resp, nil)

	resp, err := httpClient.Post("http://example.com", "application/json", bytes.NewBuffer([]byte(`{"key":"value"}`)))
	if err != nil {
		t.Fatalf("Failed to make HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status OK, got %s", resp.Status)
	}
}
