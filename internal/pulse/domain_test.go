package pulse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPulse(t *testing.T) {
	t.Run("ValidPulse", func(t *testing.T) {
		pulse, err := NewPulse("tenant1", "sku1", 100.0, KB)
		assert.NoError(t, err)
		assert.Equal(t, "tenant1", pulse.TenantId)
		assert.Equal(t, "sku1", pulse.ProductSku)
		assert.Equal(t, 100.0, pulse.UsedAmount)
		assert.Equal(t, KB, pulse.UseUnit)
	})

	t.Run("InvalidUseUnit", func(t *testing.T) {
		pulse, err := NewPulse("tenant1", "sku1", 100.0, "INVALID")
		assert.Error(t, err)
		assert.Nil(t, pulse)
		assert.Contains(t, err.Error(), "unrecognized pulse unit: INVALID")
	})
}

func TestPulseUnit_IsValid(t *testing.T) {
	tests := []struct {
		unit PulseUnit
		want bool
	}{
		{KB, true},
		{MB, true},
		{GB, true},
		{KBxSec, true},
		{MBxSec, true},
		{GBxSec, true},
		{"INVALID", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.unit), func(t *testing.T) {
			got := tt.unit.IsValid()
			assert.Equal(t, tt.want, got)
		})
	}
}