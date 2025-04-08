package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/ThalysSilva/ingestor-consumo/internal/pulseproducer"
	"github.com/rs/zerolog/log"
)

var (
	NGINX_PORT = os.Getenv("NGINX_PORT")
)

const (
	minDelay     = 100
	maxDelay     = 400
	timeDuration = 100 * time.Second
	qtyTenants   = 100
	qtySKUs      = 10
)

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("falha ao obter o caminho do arquivo atual")
		os.Exit(1)
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	clients.InitLog("log_producer.log", projectRoot)
}

func main() {
	NGINXHost := os.Getenv("NGINX_HOST")
if NGINXHost == "" {
	NGINXHost = "localhost"
}
ingestorURL := fmt.Sprintf("http://%s:%s/ingest", NGINXHost, NGINX_PORT)
	sender := pulseproducer.NewPulseProducerService(ingestorURL, minDelay, maxDelay, qtyTenants, qtySKUs)
	log.Info().Msgf("Iniciando o Envio de pulsos para %s", ingestorURL)

	sender.Start()
	time.Sleep(timeDuration)
	sender.Stop()

}
