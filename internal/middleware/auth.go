package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const (
	// UserCtxKey is the context key used to store authenticated gateway user metadata.
	UserCtxKey contextKey = "gateway_user"
)

// GatewayUser holds identity attributes forwarded by the API Gateway.
type GatewayUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// ExtractGatewayUser extracts forwarded identity headers injected by store_gateway into request context.
// Why: Enables transparent access to authenticated user attributes across downstream HTTP handlers without duplicate token parsing.
func ExtractGatewayUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-Id"))
		userEmail := strings.TrimSpace(r.Header.Get("X-User-Email"))
		userRole := strings.TrimSpace(r.Header.Get("X-User-Role"))

		if userID != "" {
			user := GatewayUser{
				ID:    userID,
				Email: userEmail,
				Role:  userRole,
			}
			ctx := context.WithValue(r.Context(), UserCtxKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAuth enforces that an authenticated user identity was injected by the API Gateway.
// Why: Prevents unauthenticated access to customer-facing order placement and history endpoints.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if !ok || strings.TrimSpace(user.ID) == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "unauthorized",
				"message": "Authentication required. Missing verified user identity.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin verifies that the incoming request was authorized by the API Gateway with an admin role.
// Why: Enforces role-based access control (RBAC) on administrative order modification and global listing endpoints.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if !ok || !strings.EqualFold(strings.TrimSpace(user.Role), "admin") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "forbidden",
				"message": "Admin privileges are required to perform this action.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext retrieves the forwarded gateway user from the request context.
// Why: Allows handlers and service layers to access the authenticated user's ID, email, and role safely.
func GetUserFromContext(ctx context.Context) (GatewayUser, bool) {
	user, ok := ctx.Value(UserCtxKey).(GatewayUser)
	return user, ok
}
