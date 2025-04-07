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

// Valida se o tipo da unidade informada é válida
func (p PulseUnit) IsValid() bool {
	validUnits := map[PulseUnit]bool{
		KB: true, MB: true, GB: true,
		KBxSec: true, MBxSec: true, GBxSec: true,
	}
	return validUnits[p]
}

type Pulse struct {
	// TenantId é o ID do cliente que está utilizando o produto
	TenantId   string    `json:"tenant_id" binding:"required"`
	// ProductSku é o SKU do produto, geralmente segue o padrão "SKU-<numero>"
	ProductSku string    `json:"product_sku" binding:"required"`
	// UsedAmount é o valor utilizado do produto
	UsedAmount float64   `json:"used_amount" binding:"required"`
	// UseUnit é a unidade utilizada para o valor utilizado do produto
	UseUnit    PulseUnit `json:"use_unit" binding:"required"`
}

// Cria um novo objeto Pulse com os parâmetros informados
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
