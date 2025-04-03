package pulse

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

type pulseHandler struct {
	pulseService PulseService
}
type PulseHandler interface {
	Ingestor() gin.HandlerFunc
}

func NewPulseHandler(pulseService PulseService) PulseHandler {
	return &pulseHandler{
		pulseService: pulseService,
	}
}

func (p *pulseHandler) Ingestor() gin.HandlerFunc {
	return func(c *gin.Context) {
		var pulso Pulse
		if err := c.ShouldBindJSON(&pulso); err != nil {
			c.Error(err)
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		p.pulseService.EnqueuePulse(pulso)
		fmt.Printf("Recebido pulso: %v\n", pulso)
		c.JSON(201, nil)
	}

}
