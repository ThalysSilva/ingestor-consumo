package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/pulsesender"
)

var (
	INGESTOR_PORT = os.Getenv("INGESTOR_PORT")
)

const (
	minDelay     = 500
	maxDelay     = 1000
	timeDuration = 100 * time.Second
	qtyTenants   = 1000
	qtySKUs      = 10
)

func main() {
	ingestorURL := fmt.Sprintf("http://localhost:%s/ingest", INGESTOR_PORT)
	sender := pulsesender.NewPulseSenderService(ingestorURL, minDelay, maxDelay, qtyTenants, qtySKUs)
	fmt.Println("Iniciando o Envio de pulsos...")

	sender.Start()
	time.Sleep(timeDuration)
	sender.Stop()

}
