package pulse

import "fmt"

type PulseUnit string

const (
	KB     PulseUnit = "KB"
	MB     PulseUnit = "MB"
	GB     PulseUnit = "GB"
	KBxSec PulseUnit = "KB/sec"
	MBxSec PulseUnit = "MB/sec"
	GBxSec PulseUnit = "GB/sec"
)

// Valida se a unidade informada é válida
func (p PulseUnit) IsValid() bool {
	validUnits := map[PulseUnit]bool{
		KB: true, MB: true, GB: true,
		KBxSec: true, MBxSec: true, GBxSec: true,
	}
	return validUnits[p]
}

type Pulse struct {
	TenantId   string    `json:"tenant_id"`
	ProductSku string    `json:"product_sku"`
	UsedAmount float64   `json:"used_amount"`
	UseUnit    PulseUnit `json:"use_unit"`
}

// Construtor de Pulse
func NewPulse(tenantId, productSku string, usedAmount float64, useUnit PulseUnit) (*Pulse, error) {
	if !useUnit.IsValid() {
		return nil, fmt.Errorf("unrecognized pulse unit: %s", useUnit)
	}
	return &Pulse{
		TenantId:   tenantId,
		ProductSku: productSku,
		UsedAmount: usedAmount,
		UseUnit:    useUnit,
	}, nil
}
