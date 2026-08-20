package ratelimit

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// slidingWindowLua is an atomic Lua script executed in Redis to compute sliding window rate limits via Sorted Sets.
// Why: Guarantees atomic eviction of expired timestamps and accurate count increment in a single network roundtrip.
const slidingWindowLua = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
local clear_before = now - window

-- 1. Remove timestamps that fall outside the sliding window
redis.call('ZREMRANGEBYSCORE', key, '-inf', clear_before)

-- 2. Count valid requests within the current window
local current_count = redis.call('ZCARD', key)

-- 3. Check quota threshold
if current_count < limit then
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, window)
    return {1, limit - current_count - 1, 0}
else
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_after_ms = 1000
    if #oldest >= 2 then
        local oldest_ts = tonumber(oldest[2])
        retry_after_ms = (oldest_ts + window) - now
        if retry_after_ms < 0 then
            retry_after_ms = 0
        end
    end
    local retry_after_sec = math.ceil(retry_after_ms / 1000)
    if retry_after_sec < 1 then
        retry_after_sec = 1
    end
    return {0, 0, retry_after_sec}
end
`

// Result encapsulates the outcome of a rate limit evaluation.
type Result struct {
	Allowed       bool
	Remaining     int
	Limit         int
	RetryAfterSec int
	Degraded      bool
}

// Limiter defines the contract for checking request quotas.
// Why: Decouples rate evaluation from HTTP handlers, enabling seamless unit testing and mock substitution.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (*Result, error)
	Close() error
}

// RedisLimiter implements the Limiter interface using a Redis Sorted Set (ZSET) backend.
type RedisLimiter struct {
	client  *redis.Client
	script  *redis.Script
	timeout time.Duration
	enabled bool
}

// NewRedisLimiter initializes a RedisLimiter with connection pooling and script pre-compilation.
// Why: Pre-hashes Lua script SHA on Redis boot to maximize throughput and minimize payload size on every request.
func NewRedisLimiter(client *redis.Client, enabled bool, timeout time.Duration) *RedisLimiter {
	if timeout <= 0 {
		timeout = 25 * time.Millisecond
	}
	return &RedisLimiter{
		client:  client,
		script:  redis.NewScript(slidingWindowLua),
		timeout: timeout,
		enabled: enabled,
	}
}

// Allow evaluates whether a given key has exceeded its allowed request quota within a sliding time window.
// Why: Provides non-blocking, fast-path rate checking with fail-open fallback to preserve service uptime during cache outages.
func (r *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (*Result, error) {
	// If rate limiting is globally disabled via configuration, allow immediately
	if !r.enabled || r.client == nil {
		return &Result{
			Allowed:       true,
			Remaining:     limit,
			Limit:         limit,
			RetryAfterSec: 0,
			Degraded:      false,
		}, nil
	}

	evalCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	nowMs := time.Now().UnixMilli()
	windowMs := window.Milliseconds()
	member := fmt.Sprintf("%d-%s", nowMs, uuid.NewString())

	res, err := r.script.Run(evalCtx, r.client, []string{key}, nowMs, windowMs, limit, member).Result()
	if err != nil {
		// Fail-open: log degraded state and permit request
		log.Printf("[WARN] Redis rate limiter check failed for key '%s': %v. Failing open.", key, err)
		return &Result{
			Allowed:       true,
			Remaining:     limit,
			Limit:         limit,
			RetryAfterSec: 0,
			Degraded:      true,
		}, nil
	}

	values, ok := res.([]interface{})
	if !ok || len(values) < 3 {
		log.Printf("[WARN] Unexpected rate limiter script result format: %v. Failing open.", res)
		return &Result{
			Allowed:       true,
			Remaining:     limit,
			Limit:         limit,
			RetryAfterSec: 0,
			Degraded:      true,
		}, nil
	}

	allowedVal, _ := values[0].(int64)
	remainingVal, _ := values[1].(int64)
	retryAfterVal, _ := values[2].(int64)

	return &Result{
		Allowed:       allowedVal == 1,
		Remaining:     int(remainingVal),
		Limit:         limit,
		RetryAfterSec: int(retryAfterVal),
		Degraded:      false,
	}, nil
}

// Close terminates the underlying Redis client connection pool.
// Why: Ensures clean release of active sockets on server shutdown.
func (r *RedisLimiter) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
