package handler

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse standardized JSON error envelope.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// RespondJSON writes a JSON response with status code.
// Why: Standardizes Content-Type header setting and JSON payload serialization.
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// RespondError writes a structured JSON error response.
// Why: Provides consistent error format across all HTTP endpoints in the ecosystem.
func RespondError(w http.ResponseWriter, status int, errType, message string) {
	RespondJSON(w, status, ErrorResponse{
		Error:   errType,
		Message: message,
	})
}

// RespondValidationError writes validation error details.
// Why: Details field-level input validation failures for client-side forms.
func RespondValidationError(w http.ResponseWriter, details map[string]string) {
	RespondJSON(w, http.StatusBadRequest, ErrorResponse{
		Error:   "validation_failed",
		Message: "One or more request parameters failed validation.",
		Details: details,
	})
}
