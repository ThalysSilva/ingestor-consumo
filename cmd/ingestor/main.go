package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)


var (
	INGESTOR_PORT = os.Getenv("INGESTOR_PORT")
	REDIS_PORT    = os.Getenv("REDIS_PORT")
	REDIS_HOST    = os.Getenv("REDIS_HOST")
)

var (
	ctx         = context.Background()
	redisClient = redis.NewClient(&redis.Options{Addr: REDIS_HOST + ":" + REDIS_PORT})
)

func main() {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer redisClient.Close()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	pulseService := pulse.NewPulseService(ctx, redisClient)
	pulseService.Start(5) 

	r := gin.Default()
	r.POST("/ingest", pulse.HandleIngestor(pulseService))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	go func() {
		fmt.Printf("Servidor rodando em :%s e métricas em :%s/metrics\n", INGESTOR_PORT, INGESTOR_PORT)
		if err := r.Run(":" + INGESTOR_PORT); err != nil {

			fmt.Printf("Erro ao iniciar o servidor: %v\n", err)
			os.Exit(1)
			return
		}
	}()

	<-stop
	fmt.Println("Recebido sinal de parada, finalizando...")

	// Para os workers corretamente
	pulseService.Stop()
	fmt.Println("Todos os workers pararam.")
}
