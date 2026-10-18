package middleware

import (
	"container/list"
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
// seen identifier is evicted and the new one admitted, so a flood of new
// clients costs fixed memory and never locks out a legitimate client.
const rateLimiterMaxVisitors = 10_000

// rateLimiterVisitor pairs a per-identifier token-bucket limiter with the
// last time it was touched, so idle identifiers can be evicted.
type rateLimiterVisitor struct {
	id       string
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiterStore is a minimal in-memory per-identifier limiter store. It
// exists (rather than reusing echo/middleware's built-in RateLimiterMemoryStore)
// because that store doesn't expose the underlying limiter, and reporting
// X-RateLimit-Remaining requires reading it. Swap this for a distributed
// store when running multiple instances.
//
// lru orders visitors by lastSeen, most recent at the front, so both the
// cap eviction and the idle sweep take the oldest entries from the back
// without scanning the map.
type rateLimiterStore struct {
	mu        sync.Mutex
	visitors  map[string]*list.Element
	lru       *list.List
	rateLimit rate.Limit
	burst     int
	expiresIn time.Duration
	max       int
}

func newRateLimiterStore(r rate.Limit, burst int, expiresIn time.Duration) *rateLimiterStore {
	s := &rateLimiterStore{
		visitors:  make(map[string]*list.Element),
		lru:       list.New(),
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

	if el, ok := s.visitors[identifier]; ok {
		v := el.Value.(*rateLimiterVisitor)
		v.lastSeen = time.Now()
		s.lru.MoveToFront(el)
		return v.limiter
	}

	if s.lru.Len() >= s.max {
		s.removeLocked(s.lru.Back())
	}
	v := &rateLimiterVisitor{id: identifier, limiter: rate.NewLimiter(s.rateLimit, s.burst), lastSeen: time.Now()}
	s.visitors[identifier] = s.lru.PushFront(v)
	return v.limiter
}

func (s *rateLimiterStore) removeLocked(el *list.Element) {
	delete(s.visitors, el.Value.(*rateLimiterVisitor).id)
	s.lru.Remove(el)
}

// ipv6ClientMask is the prefix one IPv6 client is assumed to hold. An ISP
// routinely hands a single subscriber a whole /64, so keying on the full
// address would give that client 2^64 fresh budgets.
var ipv6ClientMask = net.CIDRMask(64, 128)

// clientIP returns the rate-limit identity for c.RealIP(): an IPv4 address
// whole, an IPv6 address as its /64. Anything that is not an address, such as
// a raw header value Echo passed through, falls back to the TCP peer, so a
// store key is never longer than an address.
func clientIP(c echo.Context) string {
	ip := net.ParseIP(c.RealIP())
	if ip == nil {
		host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
		if err != nil {
			return c.Request().RemoteAddr
		}
		if ip = net.ParseIP(host); ip == nil {
			return host
		}
	}
	if ip.To4() != nil {
		return ip.String()
	}
	return ip.Mask(ipv6ClientMask).String() + "/64"
}

// evictLoop periodically drops identifiers idle longer than expiresIn so the
// store doesn't grow unbounded under many distinct clients.
func (s *rateLimiterStore) evictLoop() {
	ticker := time.NewTicker(s.expiresIn)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-s.expiresIn)
		s.mu.Lock()
		for el := s.lru.Back(); el != nil && el.Value.(*rateLimiterVisitor).lastSeen.Before(cutoff); el = s.lru.Back() {
			s.removeLocked(el)
		}
		s.mu.Unlock()
	}
}

// RateLimiter returns a per-client-IP rate limiter sized for authentication
// endpoints (registration, login) to blunt credential brute-forcing. It uses an
// in-memory store; swap the store for a distributed one when running multiple
// instances. When rate_limit.enabled is false, it returns a no-op middleware.
//
// Every response carries X-RateLimit-Limit / X-RateLimit-Remaining so a
// well-behaved client can back off before being rejected, and a 429 carries
// Retry-After computed from the token bucket's actual refill delay.
//
// A bucket is keyed by the method, the route and the client: the IPv4 address
// or the IPv6 /64 of c.RealIP(). One instance mounted on several route rows
// gives each row its own budget, and clients that share a /64 share a budget.
func RateLimiter(cfg appconfig.Validated) echo.MiddlewareFunc {
	mw, _ := rateLimiter(cfg.Get().RateLimit)
	return mw
}

func rateLimiter(cfg appconfig.RateLimit) (echo.MiddlewareFunc, *rateLimiterStore) {
	if !cfg.Enabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}, nil
	}

	store := newRateLimiterStore(rate.Limit(cfg.Rate), cfg.Burst, cfg.ExpiresIn)
	limitHeader := strconv.Itoa(cfg.Burst)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			limiter := store.getLimiter(c.Request().Method + " " + c.Path() + " " + clientIP(c))

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
