package http

import (
	"fmt"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"net"
	"net/http"
	"slices"

	_ "go-echo-boilerplate/docs"

	"github.com/jeremygprawira/wlog"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

type bearerHandler func(c echo.Context, caller jwtc.Claims) error

type route struct {
	method      string
	path        string
	public      echo.HandlerFunc
	bearer      bearerHandler
	rateLimited bool
	idempotent  bool
}

func NewServer(
	cfg config.Validated,
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
	app := cfg.Get().Application
	clientIP := ipExtractor(app.TrustedProxies)
	e.IPExtractor = clientIP

	e.Server.ReadHeaderTimeout = app.ReadHeaderTimeout
	e.Server.ReadTimeout = app.ReadTimeout
	e.Server.WriteTimeout = app.WriteTimeout
	e.Server.IdleTimeout = app.IdleTimeout

	mw := middleware.New(e, cfg, log)
	mw.Default()

	e.GET("/", func(ctx echo.Context) error {
		return ctx.String(http.StatusOK, "This is your Go Echo Boilerplate")
	})
	registerDocsRoutes(e, app.Environment)
	e.GET("/health", healthHandler{health: health}.Check)

	controls := controls{
		rateLimit:   middleware.RateLimiter(cfg, clientIP),
		bearer:      middleware.BearerAuthMiddleware(jwt, revocation),
		idempotency: middleware.Idempotency(cfg.Get().Idempotency.TTL),
	}
	controls.mount(e.Group("/api", mw.ApiKeyMiddleware()), slices.Concat(
		userRoutes(users),
	))
	return e
}

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

func registerDocsRoutes(eco *echo.Echo, env config.Environment) {
	if env.IsProduction() {
		return
	}
	eco.GET("/docs", func(ectx echo.Context) error {
		return ectx.File("api-docs.html")
	})
	eco.GET("/swagger/*", echoSwagger.WrapHandler)
}

// ipExtractor returns the TCP peer when no proxies are trusted, and ignores
// client headers. Otherwise X-Forwarded-For is read right to left, skipping only the
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
