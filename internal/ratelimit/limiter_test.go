package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func TestRedisLimiter_Disabled(t *testing.T) {
	limiter := NewRedisLimiter(nil, false, 25*time.Millisecond)
	res, err := limiter.Allow(context.Background(), "test_key", 5, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Errorf("expected request to be allowed when disabled")
	}
	if res.Degraded {
		t.Errorf("expected degraded to be false when cleanly disabled")
	}
	if res.Remaining != 5 {
		t.Errorf("expected remaining to be 5, got %d", res.Remaining)
	}
}

func TestRedisLimiter_FailOpenOnError(t *testing.T) {
	// Point to unreachable Redis address with short timeout
	invalidClient := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:59999",
		DialTimeout: 5 * time.Millisecond,
	})
	defer invalidClient.Close()

	limiter := NewRedisLimiter(invalidClient, true, 10*time.Millisecond)
	res, err := limiter.Allow(context.Background(), "test_key", 3, 10*time.Second)

	if err != nil {
		t.Fatalf("expected nil error on fail-open, got %v", err)
	}
	if !res.Allowed {
		t.Errorf("expected request to be allowed on fail-open")
	}
	if !res.Degraded {
		t.Errorf("expected degraded flag to be true on Redis connection failure")
	}
}

func TestLiveRedis_SlidingWindowQuota(t *testing.T) {
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
		t.Skipf("Skipping live Redis test: Redis instance unreachable at %s (%v)", redisAddr, err)
		return
	}

	testKey := "ratelimit:test:sliding_window_user_1"
	_ = client.Del(context.Background(), testKey).Err()
	defer func() {
		_ = client.Del(context.Background(), testKey).Err()
	}()

	limiter := NewRedisLimiter(client, true, 50*time.Millisecond)

	// Threshold: 3 requests per 2 seconds
	limit := 3
	window := 2 * time.Second

	// 1st request -> Allowed (Remaining: 2)
	res1, err := limiter.Allow(context.Background(), testKey, limit, window)
	if err != nil || !res1.Allowed || res1.Remaining != 2 {
		t.Fatalf("req 1 failed: allowed=%v, remaining=%d, err=%v", res1.Allowed, res1.Remaining, err)
	}

	// 2nd request -> Allowed (Remaining: 1)
	res2, err := limiter.Allow(context.Background(), testKey, limit, window)
	if err != nil || !res2.Allowed || res2.Remaining != 1 {
		t.Fatalf("req 2 failed: allowed=%v, remaining=%d, err=%v", res2.Allowed, res2.Remaining, err)
	}

	// 3rd request -> Allowed (Remaining: 0)
	res3, err := limiter.Allow(context.Background(), testKey, limit, window)
	if err != nil || !res3.Allowed || res3.Remaining != 0 {
		t.Fatalf("req 3 failed: allowed=%v, remaining=%d, err=%v", res3.Allowed, res3.Remaining, err)
	}

	// 4th request -> Rejected! (Allowed: false, RetryAfter > 0)
	res4, err := limiter.Allow(context.Background(), testKey, limit, window)
	if err != nil {
		t.Fatalf("req 4 unexpected error: %v", err)
	}
	if res4.Allowed {
		t.Fatalf("req 4 should have been blocked by rate limit, but was allowed")
	}
	if res4.RetryAfterSec <= 0 {
		t.Fatalf("req 4 expected RetryAfterSec > 0, got %d", res4.RetryAfterSec)
	}
	t.Logf("Rate limiting blocked 4th request successfully! RetryAfter: %ds", res4.RetryAfterSec)
}

