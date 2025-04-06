package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/mocks"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	API_URL_SENDER = os.Getenv("API_URL_SENDER")
	INGESTOR_PORT  = os.Getenv("INGESTOR_PORT")
	REDIS_PORT     = os.Getenv("REDIS_PORT")
	REDIS_HOST     = os.Getenv("REDIS_HOST")
)

var (
	ctx            = context.Background()
	mockHTTPClient = mocks.NewHttpClientMock(200, nil, nil)
)

func init() {
	clients.InitLog("log_ingestor.log")
}

func main() {
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
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

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
