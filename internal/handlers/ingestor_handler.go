package handlers

import (
	"fmt"

	"github.com/ThalysSilva/ingestor-consumo/internal/entities"
	"github.com/ThalysSilva/ingestor-consumo/internal/services/pulse"
	"github.com/gin-gonic/gin"
)

func Ingestor(pulseService pulse.PulseService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var pulso entities.Pulse
		if err := c.ShouldBindJSON(&pulso); err != nil {
			c.Error(err)
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		pulseService.EnqueuePulse(pulso)
		fmt.Printf("Recebido pulso: %v\n", pulso)
		// no content return
		c.JSON(201, nil)
	}

}
