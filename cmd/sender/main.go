// cmd/pulse-sender/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func main() {
	ctx := context.Background()
	mockHTTPClient := NewHttpClientMock(200, nil, nil)
	sentinelAddrs := []string{
		"redis-sentinel-1:26379",
		"redis-sentinel-2:26379",
		"redis-sentinel-3:26379",
	}

	redisClient := clients.InitRedisClient(REDIS_HOST, REDIS_PORT, sentinelAddrs)
	defer redisClient.Close()

	pulseSender := pulsesender.NewPulseSenderService(ctx, redisClient, API_URL_SENDER, 500, pulsesender.WithCustomHTTPClient(mockHTTPClient))
	go pulseSender.StartLoop(1*time.Minute, 5*time.Second)

	r := gin.Default()
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	log.Info().Msgf("PulseSender rodando em :%s/metrics", PULSE_SENDER_PORT)
	r.Run(":" + PULSE_SENDER_PORT)
}
