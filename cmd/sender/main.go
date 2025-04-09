// cmd/pulse-sender/main.go
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulsesender"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	API_URL_SENDER    = os.Getenv("API_URL_SENDER")
	REDIS_PORT        = os.Getenv("REDIS_PORT")
	REDIS_HOST        = os.Getenv("REDIS_HOST")
	PULSE_SENDER_PORT = os.Getenv("PULSE_SENDER_PORT")
)

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("falha ao obter o caminho do arquivo atual")
		os.Exit(1)
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	clients.InitLog("log_pulse_sender.log", projectRoot)
}

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

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("falha ao obter o caminho do arquivo atual")
		os.Exit(1)
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	clients.InitLog("log_sender.log", projectRoot)
}

func main() {
	ctx := context.Background()
	mockHTTPClient := NewHttpClientMock(200, nil, nil)
	sentinelEnv := os.Getenv("REDIS_SENTINEL_ADDRS")
	sentinelAddrs := strings.Split(sentinelEnv, ",")

	redisClient := clients.InitRedisClient(REDIS_HOST, REDIS_PORT, sentinelAddrs)
	defer redisClient.Close()

	pulseSender := pulsesender.NewPulseSenderService(ctx, redisClient, API_URL_SENDER, 500, pulsesender.WithCustomHTTPClient(mockHTTPClient))
	go pulseSender.StartLoop(1*time.Minute, 5*time.Second)

	r := gin.Default()
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	log.Info().Msgf("PulseSender rodando em :%s/metrics", PULSE_SENDER_PORT)
	r.Run(":" + PULSE_SENDER_PORT)
}
