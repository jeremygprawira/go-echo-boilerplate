package v1

import (
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/constant"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/response"
	"go-echo-boilerplate/internal/pkg/validator"
	"go-echo-boilerplate/internal/service"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type userV1Handler struct {
	service    *service.Service
	config     *config.Configuration
	jwtConfig  *jwtc.Configuration
	revocation middleware.RevocationChecker
}

func NewUserV1(v1 *echo.Group, service *service.Service, config *config.Configuration, jwtConfig *jwtc.Configuration, revocation middleware.RevocationChecker) {
	h := &userV1Handler{
		service:    service,
		config:     config,
		jwtConfig:  jwtConfig,
		revocation: revocation,
	}

	// Config.Validate rejects a bad ttl at startup, so the parse cannot fail
	// here. A nil config (handler tests) leaves the ttl at zero.
	var idempotencyTTL time.Duration
	if config != nil {
		idempotencyTTL, _ = time.ParseDuration(config.Idempotency.TTL)
	}

	noBearerRoute := v1.Group("/users")
	noBearerRoute.POST("", h.Create, middleware.RateLimiter(config), middleware.Idempotency(idempotencyTTL))
	noBearerRoute.POST("/tokens", h.GetTokens, middleware.RateLimiter(config))
	// No Idempotency here: a cached replay would skip the revocation check and
	// hand out the tokens from an already rotated refresh token.
	noBearerRoute.POST("/tokens/refresh", h.RefreshTokens, middleware.RateLimiter(config))

	bearerRoute := v1.Group("/users")
	bearerRoute.Use(middleware.BearerAuthMiddleware(h.jwtConfig, h.revocation))
	bearerRoute.GET("/me", h.GetUserByAccessToken)
	bearerRoute.POST("/logout", h.Logout)
}

// Create registers a new user
// @Summary Create New User
// @Description Register a new user with email, phone number, and password. Auto-generates account number. Optionally pass an Idempotency-Key header to make a retried request safe: a repeated request with the same key returns the original response instead of re-executing.
// @Tags Users
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Optional key making a retried request safe to resend"
// @Param request body models.CreateUserRequest true "User Registration Details"
// @Success 201 {object} models.Response{data=models.CreateUserResponse} "User Created Successfully"
// @Failure 400 {object} models.Response "Invalid Input"
// @Failure 422 {object} models.Response "Validation Failed"
// @Failure 409 {object} models.Response "User Already Exists (Email or Phone)"
// @Failure 500 {object} models.Response "Internal Server Error"
// @Router /api/v1/users [post]
func (h *userV1Handler) Create(ctx echo.Context) error {
	var request models.CreateUserRequest
	if err := ctx.Bind(&request); err != nil {
		return err
	}

	if err := validator.Input(request); err != nil {
		return errorc.FromValidation(err)
	}

	user, err := h.service.User.Create(ctx.Request().Context(), &request)
	if err != nil {
		return err
	}

	return response.Success(ctx, http.StatusCreated, user.CreateUserResponse())
}

// GetTokens retrieves tokens for a user
// @Summary Get Tokens
// @Description Get tokens for a user
// @Tags Users
// @Accept json
// @Produce json
// @Param request body models.GetUserTokenRequest true "User Token Request"
// @Success 200 {object} models.Response{data=models.GetUserTokenResponse} "User Tokens Retrieved Successfully"
// @Failure 400 {object} models.Response "Invalid Input"
// @Failure 422 {object} models.Response "Validation Failed"
// @Failure 404 {object} models.Response "User Not Found"
// @Failure 500 {object} models.Response "Internal Server Error"
// @Router /api/v1/users/tokens [post]
func (h *userV1Handler) GetTokens(ctx echo.Context) error {
	var request models.GetUserTokenRequest
	if err := ctx.Bind(&request); err != nil {
		return err
	}

	if err := validator.Input(request); err != nil {
		return errorc.FromValidation(err)
	}

	user, err := h.service.User.GetTokens(ctx.Request().Context(), &request)
	if err != nil {
		return err
	}

	ExpiresAt := time.Now().Add(time.Second * time.Duration(user.Tokens[1].ExpiredIn))

	ctx.SetCookie(&http.Cookie{
		Name:     constant.RefreshTokenCookieName,
		Value:    user.Tokens[1].Token,
		Expires:  ExpiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	return response.Success(ctx, http.StatusOK, user)
}

// RefreshTokens rotates a refresh token for a fresh access+refresh pair. The
// token is read from the request body if present, else from the
// refresh_token cookie set at login — see jwtc.ResolveRefreshToken.
// @Summary Refresh Tokens
// @Description Exchange a valid, non-revoked refresh token for a new access+refresh pair. The token may be supplied in the body or via the refresh_token cookie set at login; the body takes precedence. The presented refresh token is revoked, so a retry with it gets 401 and the client must log in again.
// @Tags Users
// @Accept json
// @Produce json
// @Param request body models.RefreshTokenRequest false "Refresh Token (optional if the refresh_token cookie is set)"
// @Success 200 {object} models.Response{data=models.GetUserTokenResponse} "Tokens Refreshed Successfully"
// @Failure 400 {object} models.Response "Invalid Input"
// @Failure 422 {object} models.Response "Validation Failed"
// @Failure 401 {object} models.Response "Invalid or Revoked Refresh Token"
// @Router /api/v1/users/tokens/refresh [post]
func (h *userV1Handler) RefreshTokens(ctx echo.Context) error {
	var request models.RefreshTokenRequest
	if err := ctx.Bind(&request); err != nil {
		return err
	}

	refreshToken := jwtc.ResolveRefreshToken(ctx.Request(), request.RefreshToken)
	if refreshToken == "" {
		return errorc.Validation.New().FieldError("refreshToken", "MISSING_FIELD", "refreshToken is required")
	}

	tokens, err := h.service.User.RefreshTokens(ctx.Request().Context(), refreshToken)
	if err != nil {
		return err
	}

	return response.Success(ctx, http.StatusOK, tokens)
}

// Logout revokes the caller's access token, and the refresh token — read from
// the request body if present, else the refresh_token cookie — so neither is
// accepted again before their natural expiry. See jwtc.ResolveRefreshToken.
// @Summary Logout
// @Description Revoke the current access token and refresh token. The refresh token may be supplied in the body or via the refresh_token cookie set at login; the body takes precedence.
// @Tags Users
// @Accept json
// @Produce json
// @Param request body models.LogoutRequest false "Refresh Token (optional if the refresh_token cookie is set)"
// @Success 200 {object} models.Response "Logged Out Successfully"
// @Failure 401 {object} models.Response "Unauthorized"
// @Router /api/v1/users/logout [post]
// @Security BearerAuth
func (h *userV1Handler) Logout(ctx echo.Context) error {
	jti, _ := ctx.Get("jti").(string)

	var request models.LogoutRequest
	_ = ctx.Bind(&request) // body is optional; the cookie fallback covers callers with none

	refreshToken := jwtc.ResolveRefreshToken(ctx.Request(), request.RefreshToken)

	if err := h.service.User.Logout(ctx.Request().Context(), jti, refreshToken); err != nil {
		return err
	}

	ctx.SetCookie(&http.Cookie{
		Name:     constant.RefreshTokenCookieName,
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	return response.Success(ctx, http.StatusOK, nil)
}

// GetUserByAccessToken retrieves user information by access token
// @Summary Get User By Access Token
// @Description Get user information by access token
// @Tags Users
// @Accept json
// @Produce json
// @Success 200 {object} models.Response{data=models.GetUserByAccountNumberResponse} "User Information Retrieved Successfully"
// @Failure 401 {object} models.Response "Unauthorized"
// @Failure 404 {object} models.Response "User Not Found"
// @Failure 500 {object} models.Response "Internal Server Error"
// @Router /api/v1/users/me [get]
// @Security BearerAuth
func (h *userV1Handler) GetUserByAccessToken(ctx echo.Context) error {
	accountNumber := ctx.Get("accountNumber").(string)

	user, err := h.service.User.GetByAccountNumber(ctx.Request().Context(), accountNumber)
	if err != nil {
		return err
	}

	return response.Success(ctx, http.StatusOK, user.GetUserByAccountNumberResponse())
}
