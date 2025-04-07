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

// Ingestor é o handler que recebe as requisições de ingestão de pulsos
// e os processa. Ele espera um JSON com os campos TenantId, ProductSku, UsedAmount e UseUnit.
// O campo UseUnit deve ser um dos seguintes: KB, MB, GB, KBxSec, MBxSec ou GBxSec.
// @OperationId Ingestor
// @Summary Ingestor de pulsos
// @Description Ingestor de pulsos
// @Tags Pulso
// @Accept json
// @Produce json
// @Param pulse body Pulse true "Pulse"
// @Success 204 {object} nil "No Content"
// @Router /pulse/ingestor [post]
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
