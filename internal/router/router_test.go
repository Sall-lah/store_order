package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Sall-lah/store_order/internal/config"
	"github.com/Sall-lah/store_order/internal/handler"
	"github.com/Sall-lah/store_order/internal/ratelimit"
	"github.com/Sall-lah/store_order/internal/router"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type dummyLimiter struct {
	allowed bool
}

func (d *dummyLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (*ratelimit.Result, error) {
	if !d.allowed {
		return &ratelimit.Result{
			Allowed:       false,
			Remaining:     0,
			Limit:         limit,
			RetryAfterSec: 5,
			Degraded:      false,
		}, nil
	}
	return &ratelimit.Result{
		Allowed:       true,
		Remaining:     limit - 1,
		Limit:         limit,
		RetryAfterSec: 0,
		Degraded:      false,
	}, nil
}

func (d *dummyLimiter) Close() error {
	return nil
}

func TestRouter_RateLimiting_Rejection(t *testing.T) {
	cfg := &config.Config{
		Port:       "8060",
		Dev:        true,
		EnableDocs: false,
	}

	mockLimit := &dummyLimiter{allowed: false}

	r := router.SetupRouter(router.RouterDeps{
		Config:         cfg,
		OrderHandler:   handler.NewOrderHandler(nil),
		WebhookHandler: handler.NewWebhookHandler(nil),
		AdminHandler:   handler.NewAdminHandler(nil),
		DevHandler:     handler.NewDevHandler(nil),
		HealthHandler:  handler.NewHealthHandler(nil),
		RateLimiter:    mockLimit,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/webhook/midtrans", nil)
	req.Header.Set("X-Real-IP", "1.1.1.1")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 Too Many Requests, got %d", rec.Code)
	}

	if rec.Header().Get("Retry-After") != "5" {
		t.Errorf("expected Retry-After header '5', got '%s'", rec.Header().Get("Retry-After"))
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if body["error"] != "rate_limit_exceeded" {
		t.Errorf("expected error 'rate_limit_exceeded', got '%s'", body["error"])
	}
}

func TestRouter_LiveRedis_SlidingWindowIntegration(t *testing.T) {
	_ = godotenv.Load("../../.env", ".env")

	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPass := os.Getenv("REDIS_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPass,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping live Redis router test: Redis unreachable at %s (%v)", redisAddr, err)
		return
	}

	limiter := ratelimit.NewRedisLimiter(client, true, 50*time.Millisecond)

	cfg := &config.Config{
		Port:       "8060",
		Dev:        true,
		EnableDocs: false,
	}

	r := router.SetupRouter(router.RouterDeps{
		Config:         cfg,
		OrderHandler:   handler.NewOrderHandler(nil),
		WebhookHandler: handler.NewWebhookHandler(nil),
		AdminHandler:   handler.NewAdminHandler(nil),
		DevHandler:     handler.NewDevHandler(nil),
		HealthHandler:  handler.NewHealthHandler(nil),
		RateLimiter:    limiter,
	})

	// Clear test user prefix in Redis
	testUserID := "usr_test_integration_flow"
	_ = client.Del(context.Background(), "ratelimit:checkout:user:"+testUserID)

	// Send requests to /api/v1/orders (limit is 3 requests / 10s for checkout)
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
		req.Header.Set("X-User-Id", testUserID)
		req.Header.Set("X-User-Role", "CUSTOMER")
		req.Header.Set("X-User-Email", "test@store.com")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		// Handler returns 400 because body is empty, but middleware permitted request past rate limiting
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was unexpectedly rate limited", i)
		}
	}

	// 4th request within 10s window must be rate limited with 429
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	req.Header.Set("X-User-Id", testUserID)
	req.Header.Set("X-User-Role", "CUSTOMER")
	req.Header.Set("X-User-Email", "test@store.com")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request expected 429 Too Many Requests, got %d", rec.Code)
	}

	t.Logf("Live Redis rate limiting integration test passed! Response status: %d, Retry-After: %s",
		rec.Code, rec.Header().Get("Retry-After"))
}
