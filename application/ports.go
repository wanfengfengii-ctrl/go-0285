// Package application hosts the command service that orchestrates the full
// precast wall grouting lifecycle: locking, mixing, leasing, continuous
// pouring, detection, re-grout generation, dual review and the irreversible
// terminal decision. It depends only on the domain and persistence boundaries
// and is exercised directly by tests via injected clocks and repositories.
package application

import "precast-wall-grout-support-release/domain"

// Clock provides a logical or wall-clock time source. Tests inject a manual
// clock; production selects logical or wall mode from configuration.
type Clock interface {
	Now() domain.LogicalTime
}

// ClockMode selects the time source behaviour for the process entry point.
type ClockMode string

const (
	ClockModeLogical ClockMode = "logical"
	ClockModeWall    ClockMode = "wall"
	ClockModeManual  ClockMode = "manual"
)

// DeviceAdapter abstracts a device so the application never treats transport
// success as business success; a test implementation uses deterministic
// scripts.
type DeviceAdapter interface {
	Call(call domain.DeviceCall) (domain.DeviceOutcome, error)
}
