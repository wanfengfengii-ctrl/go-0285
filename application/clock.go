package application

import (
	"sync"
	"time"

	"precast-wall-grout-support-release/domain"
)

// LogicalClock advances one tick per read, giving a deterministic monotonic
// time independent of the wall clock.
type LogicalClock struct {
	mu  sync.Mutex
	now int64
}

// NewLogicalClock returns a logical clock starting at zero.
func NewLogicalClock() *LogicalClock { return &LogicalClock{} }

// Now returns the current tick and advances by one.
func (c *LogicalClock) Now() domain.LogicalTime {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now++
	return domain.LogicalTime(c.now)
}

// WallClock maps the Unix nanosecond timestamp into a logical tick.
type WallClock struct{}

// NewWallClock returns a wall-clock time source.
func NewWallClock() *WallClock { return &WallClock{} }

// Now returns the current Unix nanosecond timestamp.
func (WallClock) Now() domain.LogicalTime {
	return domain.LogicalTime(time.Now().UnixNano())
}

// ManualClock is a test clock whose value is advanced explicitly.
type ManualClock struct {
	mu  sync.Mutex
	now domain.LogicalTime
}

// NewManualClock returns a manual clock at the given starting tick.
func NewManualClock(start domain.LogicalTime) *ManualClock {
	return &ManualClock{now: start}
}

// Now returns the current tick.
func (c *ManualClock) Now() domain.LogicalTime {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by n ticks.
func (c *ManualClock) Advance(n domain.LogicalTime) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now += n
}

// Set positions the clock at an absolute tick.
func (c *ManualClock) Set(t domain.LogicalTime) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}
