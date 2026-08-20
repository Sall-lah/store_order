package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sall-lah/store_order/internal/ratelimit"
)

// mockLimiter implements ratelimit.Limiter for predictable middleware testing.
type mockLimiter struct {
	allowFunc func(ctx context.Context, key string, limit int, window time.Duration) (*ratelimit.Result, error)
}

func (m *mockLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (*ratelimit.Result, error) {
	if m.allowFunc != nil {
		return m.allowFunc(ctx, key, limit, window)
	}
	return &ratelimit.Result{Allowed: true, Remaining: limit, Limit: limit}, nil
}

func (m *mockLimiter) Close() error {
	return nil
}

func TestKeyByIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name:       "From X-Forwarded-For with multiple IPs",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.195, 70.41.3.18"},
			remoteAddr: "127.0.0.1:12345",
			expected:   "ip:203.0.113.195",
		},
		{
			name:       "From X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "198.51.100.1"},
			remoteAddr: "127.0.0.1:12345",
			expected:   "ip:198.51.100.1",
		},
		{
			name:       "Fallback to RemoteAddr with port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.50:54321",
			expected:   "ip:192.168.1.50",
		},
		{
			name:       "Fallback to RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.50",
			expected:   "ip:192.168.1.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remoteAddr

			actual := KeyByIP(req)
			if actual != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, actual)
			}
		})
	}
}

func TestKeyByUser(t *testing.T) {
	t.Run("Authenticated user in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := context.WithValue(req.Context(), UserCtxKey, GatewayUser{
			ID:    "usr_abc123",
			Role:  "CUSTOMER",
			Email: "test@example.com",
		})
		req = req.WithContext(ctx)

		actual := KeyByUser(req)
		if actual != "user:usr_abc123" {
			t.Errorf("expected user:usr_abc123, got %s", actual)
		}
	})

	t.Run("Unauthenticated user falls back to IP", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "10.0.0.1")

		actual := KeyByUser(req)
		if actual != "ip:10.0.0.1" {
			t.Errorf("expected ip:10.0.0.1, got %s", actual)
		}
	})
}

func TestKeyWithPrefix(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "1.2.3.4")

	prefixedKeyer := KeyWithPrefix("checkout", KeyByIP)
	actual := prefixedKeyer(req)

	expected := "ratelimit:checkout:ip:1.2.3.4"
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestRequireRateLimit_Allowed(t *testing.T) {
	limiter := &mockLimiter{
		allowFunc: func(ctx context.Context, key string, limit int, window time.Duration) (*ratelimit.Result, error) {
			return &ratelimit.Result{
				Allowed:       true,
				Remaining:     4,
				Limit:         5,
				RetryAfterSec: 0,
				Degraded:      false,
			}, nil
		},
	}

	rule := RateLimitRule{
		Limit:  5,
		Window: time.Minute,
		Keyer:  KeyByIP,
	}

	mw := RequireRateLimit(limiter, rule)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "127.0.0.1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("expected X-RateLimit-Limit '5', got '%s'", rec.Header().Get("X-RateLimit-Limit"))
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "4" {
		t.Errorf("expected X-RateLimit-Remaining '4', got '%s'", rec.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestRequireRateLimit_Exceeded(t *testing.T) {
	limiter := &mockLimiter{
		allowFunc: func(ctx context.Context, key string, limit int, window time.Duration) (*ratelimit.Result, error) {
			return &ratelimit.Result{
				Allowed:       false,
				Remaining:     0,
				Limit:         3,
				RetryAfterSec: 7,
				Degraded:      false,
			}, nil
		},
	}

	rule := RateLimitRule{
		Limit:  3,
		Window: 10 * time.Second,
		Keyer:  KeyByUser,
	}

	mw := RequireRateLimit(limiter, rule)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/checkout", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "7" {
		t.Errorf("expected Retry-After '7', got '%s'", rec.Header().Get("Retry-After"))
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}
	if body["error"] != "rate_limit_exceeded" {
		t.Errorf("expected error 'rate_limit_exceeded', got '%s'", body["error"])
	}
}

func TestRequireRateLimit_Degraded(t *testing.T) {
	limiter := &mockLimiter{
		allowFunc: func(ctx context.Context, key string, limit int, window time.Duration) (*ratelimit.Result, error) {
			return &ratelimit.Result{
				Allowed:       true,
				Remaining:     5,
				Limit:         5,
				RetryAfterSec: 0,
				Degraded:      true,
			}, nil
		},
	}

	rule := RateLimitRule{
		Limit:  5,
		Window: time.Minute,
		Keyer:  KeyByIP,
	}

	mw := RequireRateLimit(limiter, rule)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 on degraded state, got %d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Degraded") != "true" {
		t.Errorf("expected X-RateLimit-Degraded 'true', got '%s'", rec.Header().Get("X-RateLimit-Degraded"))
	}
}

func TestRequireRateLimit_NilLimiter(t *testing.T) {
	rule := RateLimitRule{
		Limit:  5,
		Window: time.Minute,
		Keyer:  KeyByIP,
	}

	mw := RequireRateLimit(nil, rule)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 when limiter is nil, got %d", rec.Code)
	}
}
