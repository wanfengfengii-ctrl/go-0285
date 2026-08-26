package domain

import "strings"

// Code is a stable business error code. Every domain rejection maps to exactly
// one of the codes enumerated below so that clients and tests can branch on
// deterministic, human-readable identifiers rather than on message text.
type Code string

const (
	CodeWallPanelMismatch      Code = "WALL_PANEL_MISMATCH"
	CodeSocketIncompatible     Code = "SOCKET_INCOMPATIBLE"
	CodeStaleRuleDigest        Code = "STALE_RULE_DIGEST"
	CodeInvalidTopology        Code = "INVALID_TOPOLOGY"
	CodeMaterialImbalance      Code = "MATERIAL_IMBALANCE"
	CodeFixedPointOverflow     Code = "FIXED_POINT_OVERFLOW"
	CodeLeaseConflict          Code = "LEASE_CONFLICT"
	CodeLeaseExpired           Code = "LEASE_EXPIRED"
	CodeSlurryExpired          Code = "SLURRY_EXPIRED"
	CodeVolumeOverclaim        Code = "VOLUME_OVERCLAIM"
	CodePrefixViolation        Code = "PREFIX_VIOLATION"
	CodePortSealOutOfOrder     Code = "PORT_SEAL_OUT_OF_ORDER"
	CodeDeviceRetryPending     Code = "DEVICE_RETRY_PENDING"
	CodeGenerationConflict     Code = "GENERATION_CONFLICT"
	CodeIdempotencyConflict    Code = "IDEMPOTENCY_CONFLICT"
	CodeReviewerConflict       Code = "REVIEWER_CONFLICT"
	CodeEvidenceIncomplete     Code = "EVIDENCE_INCOMPLETE"
	CodeTerminalConflict       Code = "TERMINAL_CONFLICT"
	CodeConcurrentModification Code = "CONCURRENT_MODIFICATION"
	CodePersistenceCorrupt     Code = "PERSISTENCE_CORRUPT"
)

// Error is a stable domain error carrying a Code and, optionally, an ordered
// list of reasons. Batch validations collect reasons in the prescribed
// domain-key order and reject the whole command at once.
type Error struct {
	Code    Code
	Message string
	Reasons []string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(string(e.Code))
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if len(e.Reasons) > 0 {
		b.WriteString(" [")
		b.WriteString(strings.Join(e.Reasons, "; "))
		b.WriteString("]")
	}
	return b.String()
}

// NewError builds a domain error with the given code and message.
func NewError(code Code, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// WithReasons attaches an ordered list of reasons to the error.
func (e *Error) WithReasons(reasons ...string) *Error {
	if e == nil {
		return nil
	}
	e.Reasons = append([]string(nil), reasons...)
	return e
}

// IsCode reports whether err is a *Error carrying the given code.
func IsCode(err error, code Code) bool {
	de, ok := err.(*Error)
	return ok && de.Code == code
}
