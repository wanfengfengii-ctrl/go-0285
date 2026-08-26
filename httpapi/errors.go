package httpapi

import (
	"net/http"

	"precast-wall-grout-support-release/domain"
)

// statusFor maps a stable domain error code to an HTTP status. Validation and
// business rejections map to 422, races to 409, retry-pending to 202, and
// persistence corruption to 503.
func statusFor(code domain.Code) int {
	switch code {
	case domain.CodeConcurrentModification,
		domain.CodeIdempotencyConflict,
		domain.CodeLeaseConflict,
		domain.CodeGenerationConflict,
		domain.CodeTerminalConflict,
		domain.CodeReviewerConflict:
		return http.StatusConflict
	case domain.CodeDeviceRetryPending:
		return http.StatusAccepted
	case domain.CodePersistenceCorrupt:
		return http.StatusServiceUnavailable
	default:
		return http.StatusUnprocessableEntity
	}
}

// writeError writes a domain error as a stable JSON envelope.
func writeError(w http.ResponseWriter, err error) {
	de, ok := err.(*domain.Error)
	if !ok {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, statusFor(de.Code), errorBody{Error: apiError{
		Code:    de.Code,
		Message: de.Message,
		Reasons: de.Reasons,
	}})
}
