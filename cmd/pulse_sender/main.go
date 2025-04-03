package main

import (
	"fmt"
	"time"
	"github.com/ThalysSilva/ingestor-consumo/internal/services/pulsesender"
)

const (
	ingestorURL  = "http://localhost:8080/ingest"
	minDelay     = 500
	maxDelay     = 2000
	timeDuration = 10 * time.Second
	qtyTenants   = 10
)

func main() {
	sender := pulsesender.NewPulseSender(ingestorURL, minDelay, maxDelay, qtyTenants)
	fmt.Println("Iniciando o Envio de pulsos...")

	sender.Start()
	time.Sleep(timeDuration)
	sender.Stop()

}
