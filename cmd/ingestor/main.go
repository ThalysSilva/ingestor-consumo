package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	_ "github.com/ThalysSilva/ingestor-consumo/docs"
	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	API_URL_SENDER = os.Getenv("API_URL_SENDER")
	INGESTOR_PORT  = os.Getenv("INGESTOR_PORT")
	REDIS_PORT     = os.Getenv("REDIS_PORT")
	REDIS_HOST     = os.Getenv("REDIS_HOST")
)

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
	sentinelEnv := os.Getenv("REDIS_SENTINEL_ADDRS")
	sentinelAddrs := strings.Split(sentinelEnv, ",")

	redisClient := clients.InitRedisClient(REDIS_HOST, REDIS_PORT, sentinelAddrs)
	defer redisClient.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	pulseService := pulse.NewPulseService(ctx, redisClient)
	pulseHandler := pulse.NewPulseHandler(pulseService)
	go pulseService.Start(10, 5*time.Second)

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
