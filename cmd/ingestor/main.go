package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/ThalysSilva/ingestor-consumo/docs" 
)

var (
	API_URL_SENDER = os.Getenv("API_URL_SENDER")
	INGESTOR_PORT  = os.Getenv("INGESTOR_PORT")
	REDIS_PORT     = os.Getenv("REDIS_PORT")
	REDIS_HOST     = os.Getenv("REDIS_HOST")
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

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("falha ao obter o caminho do arquivo atual")
		os.Exit(1)
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	clients.InitLog("log_ingestor.log", projectRoot)
}

func main() {
	ctx := context.Background()
	mockHTTPClient := NewHttpClientMock(200, nil, nil)
	redisClient := clients.InitRedisClient(REDIS_HOST, REDIS_PORT)
	defer redisClient.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	pulseService := pulse.NewPulseService(ctx, redisClient, API_URL_SENDER, 500, pulse.WithCustomHTTPClient(mockHTTPClient))
	pulseHandler := pulse.NewPulseHandler(pulseService)
	go pulseService.Start(10, time.Minute)

	r := gin.Default()

	r.POST("/ingest", pulseHandler.Ingestor())

	// Métricas do Prometheus
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	go func() {
		// fmt.Printf("Servidor rodando em :%s e métricas em :%s/metrics\n", INGESTOR_PORT, INGESTOR_PORT)
		log.Info().Msgf("Servidor rodando em :%s e métricas em :%s/metrics\n", INGESTOR_PORT, INGESTOR_PORT)
		if err := r.Run(":" + INGESTOR_PORT); err != nil {

			log.Error().Msgf("Erro ao iniciar o servidor: %v\n", err)
			os.Exit(1)
			return
		}
	}()

	<-stop
	fmt.Println("Recebido sinal de parada, finalizando...")
	pulseService.Stop()
	fmt.Println("Todos os workers pararam.")
	cancel()

}
