package pulse

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

func HandleIngestor(pulseService PulseService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var pulso Pulse
		if err := c.ShouldBindJSON(&pulso); err != nil {
			c.Error(err)
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		pulseService.EnqueuePulse(pulso)
		fmt.Printf("Recebido pulso: %v\n", pulso)
		c.JSON(201, nil)
	}

}
