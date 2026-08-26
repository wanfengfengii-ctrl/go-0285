// Package devices provides device adapters that translate application device
// calls into scripted outcomes. The production adapter drives real devices; a
// scripted adapter supports deterministic tests without real time or network.
package devices

import (
	"sync"

	"precast-wall-grout-support-release/domain"
)

// ScriptedAdapter replays a deterministic script of outcomes for each call ID,
// falling back to success once the script is exhausted.
type ScriptedAdapter struct {
	mu      sync.Mutex
	scripts map[string][]domain.DeviceOutcome
}

// NewScriptedAdapter returns an empty scripted adapter.
func NewScriptedAdapter() *ScriptedAdapter {
	return &ScriptedAdapter{scripts: make(map[string][]domain.DeviceOutcome)}
}

// SetScript assigns an ordered outcome script to a call ID.
func (a *ScriptedAdapter) SetScript(callID string, outcomes ...domain.DeviceOutcome) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.scripts[callID] = append([]domain.DeviceOutcome(nil), outcomes...)
}

// Call consumes the next scripted outcome for the call ID.
func (a *ScriptedAdapter) Call(call domain.DeviceCall) (domain.DeviceOutcome, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	script := a.scripts[call.CallID]
	if len(script) == 0 {
		return domain.OutcomeSuccess, nil
	}
	outcome := script[0]
	a.scripts[call.CallID] = script[1:]
	return outcome, nil
}
