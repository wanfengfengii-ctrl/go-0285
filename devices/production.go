package devices

import "precast-wall-grout-support-release/domain"

// ProductionAdapter is the real-device adapter used by the production entry
// point. It represents a physical device transport that reports success; the
// application still validates the receipt independently and never treats
// transport success as a business reading. A real deployment replaces the Call
// body with an actual device protocol.
type ProductionAdapter struct{}

// NewProductionAdapter returns a production device adapter.
func NewProductionAdapter() *ProductionAdapter { return &ProductionAdapter{} }

// Call reports transport success for every device call.
func (ProductionAdapter) Call(call domain.DeviceCall) (domain.DeviceOutcome, error) {
	return domain.OutcomeSuccess, nil
}
