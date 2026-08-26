package httpapi

import "precast-wall-grout-support-release/domain"

// createTaskRequest is the body of POST /api/v1/tasks.
type createTaskRequest struct {
	TaskID    domain.TaskID `json:"taskId"`
	Building  string        `json:"building"`
	Level     string        `json:"level"`
	WallPanel string        `json:"wallPanel"`
}

// deviceAttemptRequest is the body of POST /api/v1/device-calls/{id}/attempts.
type deviceAttemptRequest struct {
	Outcome domain.DeviceOutcome `json:"outcome"`
	Value   int64                `json:"value"`
}

// errorBody is the stable JSON error envelope.
type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    domain.Code `json:"code"`
	Message string      `json:"message"`
	Reasons []string    `json:"reasons,omitempty"`
}
