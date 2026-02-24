package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// RateLimiter returns an Echo middleware that rate-limits requests using Redis.
// key is a function that derives the rate-limit key from the request context.
// limit is the number of requests allowed per window.
// window is the duration of the window.
func RateLimiter(rdb *redis.Client, keyFn func(c echo.Context) string, limit int, window time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := fmt.Sprintf("rl:%s", keyFn(c))
			ctx := c.Request().Context()

			count, err := increment(ctx, rdb, key, window)
			if err != nil {
				// On Redis failure, allow the request through (fail open)
				return next(c)
			}

			if count > int64(limit) {
				return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
			}

			return next(c)
		}
	}
}

func increment(ctx context.Context, rdb *redis.Client, key string, window time.Duration) (int64, error) {
	pipe := rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// IPKeyFn returns the remote IP address as the rate-limit key.
func IPKeyFn(c echo.Context) string {
	return c.RealIP()
}
