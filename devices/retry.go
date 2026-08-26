package devices

import "precast-wall-grout-support-release/domain"

// backoffTicks is the deterministic backoff table used to schedule device
// retries. Retry sequence numbers start at one and increase by one per attempt;
// the nextRetryAt tick is read from this fixed table, capping at the final
// entry so backoff never grows without bound.
var backoffTicks = []domain.LogicalTime{1, 2, 4, 8, 16, 32, 64}

// NextRetryAt returns the logical tick at which the next device attempt may
// run, given the current tick and the one-based attempt sequence number.
func NextRetryAt(now domain.LogicalTime, seq int) domain.LogicalTime {
	idx := seq - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(backoffTicks) {
		idx = len(backoffTicks) - 1
	}
	return now + backoffTicks[idx]
}
