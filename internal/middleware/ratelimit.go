package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sall-lah/store_order/internal/ratelimit"
)

// KeyFunc extracts a unique rate limiting key identifier from an incoming HTTP request.
// Why: Enables customizable scoping of quotas by client IP, authenticated user identity, or custom metadata.
type KeyFunc func(r *http.Request) string

// KeyByIP extracts the client IP address from proxy headers or remote address.
// Why: Enforces edge-style volumetric throttling to mitigate unauthenticated scraping or brute-force attacks.
func KeyByIP(r *http.Request) string {
	// 1. Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return "ip:" + ip
			}
		}
	}

	// 2. Check X-Real-IP
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return "ip:" + xrip
	}

	// 3. Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	if host == "" {
		host = "unknown"
	}
	return "ip:" + host
}

// KeyByUser extracts the authenticated user ID from request context, falling back to IP if unauthenticated.
// Why: Guarantees fair-share resource allocation per customer account across different client IP addresses and devices.
func KeyByUser(r *http.Request) string {
	user, ok := GetUserFromContext(r.Context())
	if ok && strings.TrimSpace(user.ID) != "" {
		return "user:" + user.ID
	}
	return KeyByIP(r)
}

// KeyWithPrefix wraps an existing KeyFunc with a specific namespace domain prefix.
// Why: Prevents quota collision across different HTTP endpoints that have distinct business thresholds (e.g. checkout vs. cancel).
func KeyWithPrefix(prefix string, keyer KeyFunc) KeyFunc {
	return func(r *http.Request) string {
		baseKey := keyer(r)
		return fmt.Sprintf("ratelimit:%s:%s", prefix, baseKey)
	}
}

// RateLimitRule defines the threshold, sliding window duration, and key resolution policy for an endpoint.
type RateLimitRule struct {
	Limit  int
	Window time.Duration
	Keyer  KeyFunc
}

// RequireRateLimit constructs an HTTP middleware that throttles requests based on the given RateLimitRule.
// Why: Shields downstream database transactions, external payment APIs, and message queues from burst traffic and bot abuse.
func RequireRateLimit(limiter ratelimit.Limiter, rule RateLimitRule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			key := rule.Keyer(r)
			res, err := limiter.Allow(r.Context(), key, rule.Limit, rule.Window)
			if err != nil {
				// Safety fallback if limiter unexpected error occurs
				next.ServeHTTP(w, r)
				return
			}

			// Inform clients of rate quota metrics
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))

			if res.Degraded {
				w.Header().Set("X-RateLimit-Degraded", "true")
			}

			if !res.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(res.RetryAfterSec))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				message := fmt.Sprintf("Too many requests. Please retry after %d seconds.", res.RetryAfterSec)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "rate_limit_exceeded",
					"message": message,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
