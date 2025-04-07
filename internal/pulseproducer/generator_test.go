package pulseproducer

import (
	"testing"

	"github.com/ThalysSilva/ingestor-consumo/internal/pulse"
	"github.com/stretchr/testify/assert"
)

func TestRandomPulse(t *testing.T) {
	t.Run("ValidPulseGeneration", func(t *testing.T) {
		skuMap := map[string]pulse.PulseUnit{
			"SKU-0": pulse.KB,
		}

		pulseProducerService := &pulseProducerService{
			skuMap: &skuMap,
		}

		pulseGenerated, err := pulseProducerService.randomPulse("tenant-123")
		if err != nil {
			t.Fatalf("Erro ao gerar pulso: %v", err)
		}

		assert.NotNil(t, pulseGenerated)
		assert.Equal(t, "tenant-123", pulseGenerated.TenantId)
		assert.Equal(t, "SKU-0", pulseGenerated.ProductSku)
		assert.GreaterOrEqual(t, pulseGenerated.UsedAmount, 0.0)
		assert.Equal(t, pulse.KB, pulseGenerated.UseUnit)
	})

	t.Run("EmptyTenantId", func(t *testing.T) {
		skuMap := map[string]pulse.PulseUnit{
			"SKU-0": pulse.KB,
		}
		pulseProducerService := &pulseProducerService{
			skuMap: &skuMap,
		}
		pulseGenerated, err := pulseProducerService.randomPulse("")
		assert.NotNil(t, err)
		assert.Nil(t, pulseGenerated)
	})

	t.Run("InvalidPulseGeneration", func(t *testing.T) {
		skuMap := map[string]pulse.PulseUnit{
			"SKU-0": "invalid-unit",
		}

		pulseProducerService := &pulseProducerService{
			skuMap: &skuMap,
		}

		pulseGenerated, err := pulseProducerService.randomPulse("tenant-123")
		assert.NotNil(t, err)
		assert.Nil(t, pulseGenerated)
	})
}

func TestRandomPulseUnit(t *testing.T) {
	t.Run("ValidPulseUnitGeneration", func(t *testing.T) {

		pulseProducerService := &pulseProducerService{}

		pulseUnitGenerated := pulseProducerService.randomPulseUnit()
		assert.Contains(t, []pulse.PulseUnit{pulse.KB, pulse.MB, pulse.GB, pulse.KBxSec, pulse.MBxSec, pulse.GBxSec}, pulseUnitGenerated)
	})
}
