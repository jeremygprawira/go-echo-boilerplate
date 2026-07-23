package middleware

import (
	"time"

	appconfig "go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// RateLimiter returns a per-client-IP rate limiter sized for authentication
// endpoints (registration, login) to blunt credential brute-forcing. It uses an
// in-memory store; swap the store for a distributed one when running multiple
// instances. When cfg.RateLimit.Enabled is false, it returns a no-op middleware
// so the limiter can be switched off without a code change. This is a
// deliberate override, not the fail-safe path: config.Initialize defaults
// rate_limit.enabled to true via viper.SetDefault, so an omitted config key
// keeps protection on; only an explicit `enabled: false` in YAML disables it.
//
// Security assumption: this relies on Echo's default identifier extractor,
// which keys off c.RealIP() and, absent a configured echo.IPExtractor, trusts
// client-supplied X-Forwarded-For / X-Real-IP headers. That means the limiter
// is only effective when the app runs behind a proxy/load balancer that
// terminates client connections and sets X-Forwarded-For / X-Real-IP itself,
// stripping any value the client tried to supply. Deployed directly
// internet-facing (no trusted proxy in front), a client can rotate these
// headers per request to obtain a fresh rate-limit bucket each time,
// defeating this protection. If that's the deployment topology, configure a
// trusted-proxy echo.IPExtractor (e.g. echo.ExtractIPFromXFFHeader with an
// explicit trusted CIDR list) before relying on this limiter.
func RateLimiter(cfg *appconfig.Configuration) echo.MiddlewareFunc {
	if cfg == nil || !cfg.RateLimit.Enabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	config := echoMiddleware.RateLimiterConfig{
		Store: echoMiddleware.NewRateLimiterMemoryStoreWithConfig(
			echoMiddleware.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(5),
				Burst:     10,
				ExpiresIn: 3 * time.Minute,
			},
		),
	}
	return echoMiddleware.RateLimiterWithConfig(config)
}
