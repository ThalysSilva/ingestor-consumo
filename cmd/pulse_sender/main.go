package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/services/pulsesender"
)

var (
	INGESTOR_PORT = os.Getenv("INGESTOR_PORT")
)

const (
	minDelay     = 500
	maxDelay     = 2000
	timeDuration = 10 * time.Second
	qtyTenants   = 1000
)

func main() {
	ingestorURL := fmt.Sprintf("http://localhost:%s/ingest", INGESTOR_PORT)
	sender := pulsesender.NewPulseSender(ingestorURL, minDelay, maxDelay, qtyTenants)
	fmt.Println("Iniciando o Envio de pulsos...")

	sender.Start()
	time.Sleep(timeDuration)
	sender.Stop()

}
