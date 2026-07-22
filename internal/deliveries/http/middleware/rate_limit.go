package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// AuthRateLimiter returns a per-client-IP rate limiter sized for authentication
// endpoints (registration, login) to blunt credential brute-forcing. It uses an
// in-memory store; swap the store for a distributed one when running multiple
// instances.
func AuthRateLimiter() echo.MiddlewareFunc {
	config := echomw.RateLimiterConfig{
		Store: echomw.NewRateLimiterMemoryStoreWithConfig(
			echomw.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(5),
				Burst:     10,
				ExpiresIn: 3 * time.Minute,
			},
		),
	}
	return echomw.RateLimiterWithConfig(config)
}
