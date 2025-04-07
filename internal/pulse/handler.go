package pulse

import (
	"github.com/gin-gonic/gin"
	"net/http"
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
		if !pulso.UseUnit.IsValid() {
			c.JSON(400, gin.H{"error": "Invalid pulse unit"})
			return
		}
		p.pulseService.EnqueuePulse(pulso)
		c.JSON(http.StatusNoContent, nil)
	}

}
