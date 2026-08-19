package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the HTTP header key for distributed request tracing.
	RequestIDHeader = "X-Request-ID"
	// RequestIDCtxKey is the context key for storing the request ID.
	RequestIDCtxKey contextKey = "request_id"
	// DefaultMaxBodyBytes limits incoming JSON payloads to 1MB.
	DefaultMaxBodyBytes = 1048576
)

// RequestID extracts the existing X-Request-ID header or generates a new UUIDv4 to preserve distributed trace context.
// Why: Enables end-to-end log correlation across API Gateway, Order Service, and Kafka message events.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get(RequestIDHeader))
		if reqID == "" {
			reqID = uuid.NewString()
		}

		w.Header().Set(RequestIDHeader, reqID)
		ctx := context.WithValue(r.Context(), RequestIDCtxKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the distributed trace request ID from context.
// Why: Enables attaching trace metadata to outgoing Kafka messages, database outbox entries, and integration logs.
func GetRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value(RequestIDCtxKey).(string); ok {
		return reqID
	}
	return ""
}

// Recoverer catches unhandled panics, logs stack traces, and sends a sanitized HTTP 500 JSON response.
// Why: Prevents HTTP server worker termination and information leakage during runtime panic scenarios.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC RECOVERED] %v\nStack:\n%s", rec, debug.Stack())

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "internal_server_error",
					"message": "An unexpected error occurred while processing your request.",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// LimitRequestBody restricts incoming request body size to guard against memory exhaustion attacks.
// Why: Enforces memory safety during JSON unmarshalling from untrusted clients.
func LimitRequestBody(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
}
