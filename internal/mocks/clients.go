package mocks

import (
	"fmt"
	"io"
	"net/http"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
)

type httpClientMock struct {
	statusCode int
	body       io.Reader
	err        error
}

func NewHttpClientMock(statusCode int, body io.Reader, err error) clients.HTTPClient {
	return &httpClientMock{
		statusCode: statusCode,
		body:       body,
		err:        err,
	}
}

func (h *httpClientMock) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	if h.err != nil {
		return nil, h.err
	}
	fmt.Print("Mocking HTTP POST request to URL: ", url)
	fmt.Print("Content-Type: ", contentType, "\n")
	// desconverter body para string para imprimir
	bodyBytes, err := io.ReadAll(body)
    if err != nil {
        return nil, fmt.Errorf("erro ao ler o body: %w", err)
    }
    bodyString := string(bodyBytes)

    fmt.Println("Body: ", bodyString)

	fmt.Print("Body: ", body, "\n")
	return &http.Response{
		StatusCode: h.statusCode,
		Body:       io.NopCloser(h.body),
	}, nil
}
