package http

import (
	"fmt"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"net"
	"net/http"
	"slices"
	"time"

	_ "go-echo-boilerplate/docs"

	"github.com/jeremygprawira/wlog"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// bearerHandler is a handler that reads its caller. Only the bearer field of a
// route takes one, so it never runs without a verified access token.
type bearerHandler func(c echo.Context, caller jwtc.Claims) error

// route is one row of the API route table. Every row is mounted under /api, so
// the API key always applies. Set exactly one of public and bearer.
type route struct {
	method string
	path   string
	// public needs only the API key.
	public echo.HandlerFunc
	// bearer also needs a live access token.
	bearer bearerHandler
	// rateLimited rows each get their own budget per client IP.
	rateLimited bool
	// idempotent rows replay the first response to a repeated Idempotency-Key.
	idempotent bool
}

// @title GO-ECHO-BOILERPLATE API DOCUMENTATION
// @version 1.0
// @description This is a go-echo-boilerplate api docs.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api
// @schemes http https

// NewServer builds the whole HTTP surface: server limits, the global
// middleware chain, the infra routes, and every API route with the controls
// its row asks for. Each feature's service is its own parameter, so wiring a
// feature into core.Setup and mounting its routes here are checked by the
// compiler together.
func NewServer(
	cfg *config.Configuration,
	log *wlog.Logger,
	jwt *jwtc.Configuration,
	revocation middleware.RevocationChecker,
	health healthService,
	users userService,
) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = ErrorHandler
	e.IPExtractor = ipExtractor(cfg.Application.TrustedProxies)

	// Config.Validate rejects a bad or non-positive value at startup. An unset
	// value (tests) parses to zero, which means no timeout.
	app := cfg.Application
	e.Server.ReadHeaderTimeout, _ = time.ParseDuration(app.ReadHeaderTimeout)
	e.Server.ReadTimeout, _ = time.ParseDuration(app.ReadTimeout)
	e.Server.WriteTimeout, _ = time.ParseDuration(app.WriteTimeout)
	e.Server.IdleTimeout, _ = time.ParseDuration(app.IdleTimeout)

	mw := middleware.New(e, cfg, log)
	mw.Default(cfg)

	e.GET("/", func(ctx echo.Context) error {
		return ctx.String(http.StatusOK, "This is your Go Echo Boilerplate")
	})
	registerDocsRoutes(e, cfg)
	e.GET("/health", healthHandler{health: health}.Check)

	// Config.Validate rejects a bad ttl at startup, so the parse cannot fail.
	idempotencyTTL, _ := time.ParseDuration(cfg.Idempotency.TTL)
	controls := controls{
		rateLimit:   middleware.RateLimiter(cfg),
		bearer:      middleware.BearerAuthMiddleware(jwt, revocation),
		idempotency: middleware.Idempotency(idempotencyTTL),
	}
	controls.mount(e.Group("/api", mw.ApiKeyMiddleware(cfg)), slices.Concat(
		userRoutes(users),
	))
	return e
}

// controls holds one instance of each per-route middleware, so every row that
// asks for a control shares it.
type controls struct {
	rateLimit, bearer, idempotency echo.MiddlewareFunc
}

func (c controls) mount(api *echo.Group, routes []route) {
	for _, r := range routes {
		if (r.public == nil) == (r.bearer == nil) {
			panic(fmt.Sprintf("route %s %s: set exactly one of public and bearer", r.method, r.path))
		}
		var chain []echo.MiddlewareFunc
		if r.rateLimited {
			chain = append(chain, c.rateLimit)
		}
		handler := r.public
		if r.bearer != nil {
			chain = append(chain, c.bearer)
			handler = middleware.WithClaims(r.bearer)
		}
		if r.idempotent {
			chain = append(chain, c.idempotency)
		}
		api.Add(r.method, r.path, handler, chain...)
	}
}

func registerDocsRoutes(eco *echo.Echo, config *config.Configuration) {
	if config.IsProduction() {
		return
	}
	eco.GET("/docs", func(ectx echo.Context) error {
		return ectx.File("api-docs.html")
	})
	eco.GET("/swagger/*", echoSwagger.WrapHandler)
}

// ipExtractor decides what c.RealIP returns, which keys the rate limiter.
// With no trusted proxies the TCP peer is the client and client headers are
// ignored. Otherwise X-Forwarded-For is read right to left, skipping only the
// listed proxies; Echo's default trust of private and loopback ranges is off.
func ipExtractor(trustedProxies []string) echo.IPExtractor {
	if len(trustedProxies) == 0 {
		return echo.ExtractIPDirect()
	}
	options := []echo.TrustOption{echo.TrustLoopback(false), echo.TrustLinkLocal(false), echo.TrustPrivateNet(false)}
	for _, cidr := range trustedProxies {
		// Config.Validate rejects a bad CIDR at startup.
		if _, ipNet, err := net.ParseCIDR(cidr); err == nil {
			options = append(options, echo.TrustIPRange(ipNet))
		}
	}
	return echo.ExtractIPFromXFFHeader(options...)
}
