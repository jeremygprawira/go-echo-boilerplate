package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	appconfig "go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// rateLimiterMaxVisitors caps the store. When it is full, the least recently
// seen identifier is evicted to make room, so a flood of new clients costs
// fixed memory instead of growing without bound.
const rateLimiterMaxVisitors = 10_000

// rateLimiterVisitor pairs a per-identifier token-bucket limiter with the
// last time it was touched, so idle identifiers can be evicted.
type rateLimiterVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiterStore is a minimal in-memory per-identifier limiter store. It
// exists (rather than reusing echo/middleware's built-in RateLimiterMemoryStore)
// because that store doesn't expose the underlying limiter, and reporting
// X-RateLimit-Remaining requires reading it. Swap this for a distributed
// store when running multiple instances.
type rateLimiterStore struct {
	mu        sync.Mutex
	visitors  map[string]*rateLimiterVisitor
	rateLimit rate.Limit
	burst     int
	expiresIn time.Duration
	max       int
}

func newRateLimiterStore(r rate.Limit, burst int, expiresIn time.Duration) *rateLimiterStore {
	s := &rateLimiterStore{
		visitors:  make(map[string]*rateLimiterVisitor),
		rateLimit: r,
		burst:     burst,
		expiresIn: expiresIn,
		max:       rateLimiterMaxVisitors,
	}
	go s.evictLoop()
	return s
}

func (s *rateLimiterStore) getLimiter(identifier string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.visitors[identifier]
	if !ok {
		if len(s.visitors) >= s.max {
			s.evictOldestLocked()
		}
		v = &rateLimiterVisitor{limiter: rate.NewLimiter(s.rateLimit, s.burst)}
		s.visitors[identifier] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// evictOldestLocked drops the least recently seen identifier. Caller holds mu.
// ponytail: linear scan, only when the store is full; use an LRU list if
// floods at the cap ever show up in profiles.
func (s *rateLimiterStore) evictOldestLocked() {
	var oldestID string
	var oldest time.Time
	for id, v := range s.visitors {
		if oldestID == "" || v.lastSeen.Before(oldest) {
			oldestID, oldest = id, v.lastSeen
		}
	}
	delete(s.visitors, oldestID)
}

// clientIP returns c.RealIP() when it is an IP address. Anything else, such as
// a raw header value Echo passed through, falls back to the TCP peer, so a
// store key is never longer than an address.
func clientIP(c echo.Context) string {
	if ip := net.ParseIP(c.RealIP()); ip != nil {
		return ip.String()
	}
	host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
	if err != nil {
		return c.Request().RemoteAddr
	}
	return host
}

// evictLoop periodically drops identifiers idle longer than expiresIn so the
// store doesn't grow unbounded under many distinct clients.
func (s *rateLimiterStore) evictLoop() {
	ticker := time.NewTicker(s.expiresIn)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-s.expiresIn)
		s.mu.Lock()
		for id, v := range s.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(s.visitors, id)
			}
		}
		s.mu.Unlock()
	}
}

// RateLimiter returns a per-client-IP rate limiter sized for authentication
// endpoints (registration, login) to blunt credential brute-forcing. It uses an
// in-memory store; swap the store for a distributed one when running multiple
// instances. When cfg.RateLimit.Enabled is false, it returns a no-op middleware
// so the limiter can be switched off without a code change. This is a
// deliberate override, not the fail-safe path: config.Initialize defaults
// rate_limit.enabled to true via viper.SetDefault, so an omitted config key
// keeps protection on; only an explicit `enabled: false` in YAML disables it.
//
// Every response carries X-RateLimit-Limit / X-RateLimit-Remaining so a
// well-behaved client can back off before being rejected, and a 429 carries
// Retry-After computed from the token bucket's actual refill delay.
//
// The client identity is c.RealIP(). core.BuildHTTPServer sets e.IPExtractor
// from application.trusted_proxies, so a client cannot pick its own bucket by
// sending X-Forwarded-For or X-Real-IP.
func RateLimiter(cfg *appconfig.Configuration) echo.MiddlewareFunc {
	mw, _ := rateLimiter(cfg)
	return mw
}

func rateLimiter(cfg *appconfig.Configuration) (echo.MiddlewareFunc, *rateLimiterStore) {
	if cfg == nil || !cfg.RateLimit.Enabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}, nil
	}

	expiresIn, err := time.ParseDuration(cfg.RateLimit.ExpiresIn)
	if err != nil {
		expiresIn = 3 * time.Minute
	}

	store := newRateLimiterStore(rate.Limit(cfg.RateLimit.Rate), cfg.RateLimit.Burst, expiresIn)
	limitHeader := strconv.Itoa(cfg.RateLimit.Burst)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			limiter := store.getLimiter(clientIP(c))

			reservation := limiter.ReserveN(time.Now(), 1)
			delay := reservation.Delay()

			remaining := int(limiter.Tokens())
			if remaining < 0 {
				remaining = 0
			}
			c.Response().Header().Set("X-RateLimit-Limit", limitHeader)
			c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if delay > 0 {
				reservation.Cancel() // don't consume a token for a request we're rejecting
				retryAfter := int(delay.Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				c.Response().Header().Set("Retry-After", strconv.Itoa(retryAfter))
				return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
			}

			return next(c)
		}
	}, store
}
