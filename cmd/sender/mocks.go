package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/rs/zerolog/log"
)

type httpClientMock struct {
	statusCode int
	body       io.Reader
	err        error
}

// NewHttpClientMock cria um novo mock de cliente HTTP
// com o statusCode, body e err especificados.
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
	log.Debug().Msgf("Mocking HTTP POST request to URL: %s", url)
	log.Debug().Msgf("Content-Type: %s", contentType)

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o body: %w", err)
	}
	bodyString := string(bodyBytes)

	log.Debug().Msgf("Body: %s", bodyString)

	return &http.Response{
		StatusCode: h.statusCode,
		Body:       io.NopCloser(h.body),
	}, nil
}
