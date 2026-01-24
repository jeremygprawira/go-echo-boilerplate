package v1

import (
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/response"
	"go-echo-boilerplate/internal/pkg/validator"
	"go-echo-boilerplate/internal/service"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type userV1Handler struct {
	service   *service.Service
	config    *config.Configuration
	jwtConfig *jwtc.Configuration
}

func NewUserV1(v1 *echo.Group, service *service.Service, config *config.Configuration, jwtConfig *jwtc.Configuration) {
	h := &userV1Handler{
		service:   service,
		config:    config,
		jwtConfig: jwtConfig,
	}

	noBearerRoute := v1.Group("/users")
	noBearerRoute.POST("", h.Create)
	noBearerRoute.POST("/tokens", h.GetTokens)

	bearerRoute := v1.Group("/users")
	bearerRoute.Use(middleware.BearerAuthMiddleware(h.jwtConfig))
	bearerRoute.GET("/me", h.GetUserByAccessToken)
}

// Create registers a new user
// @Summary Create New User
// @Description Register a new user with email, phone number, and password. Auto-generates account number.
// @Tags Users
// @Accept json
// @Produce json
// @Param request body models.CreateUserRequest true "User Registration Details"
// @Success 201 {object} models.Response{data=models.CreateUserResponse} "User Created Successfully"
// @Failure 400 {object} models.Response "Invalid Input / Validation Error"
// @Failure 409 {object} models.Response "User Already Exists (Email or Phone)"
// @Failure 500 {object} models.Response "Internal Server Error"
// @Router /api/v1/users [post]
func (h *userV1Handler) Create(ctx echo.Context) error {
	var request models.CreateUserRequest
	if err := ctx.Bind(&request); err != nil {
		return response.Error(ctx, err)
	}

	if err := validator.Input(request); err != nil {
		return response.ErrorValidation(ctx, err)
	}

	user, err := h.service.User.Create(ctx.Request().Context(), &request)
	if err != nil {
		return response.Error(ctx, err)
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
// @Failure 400 {object} models.Response "Invalid Input / Validation Error"
// @Failure 404 {object} models.Response "User Not Found"
// @Failure 500 {object} models.Response "Internal Server Error"
// @Router /api/v1/users/tokens [post]
func (h *userV1Handler) GetTokens(ctx echo.Context) error {
	var request models.GetUserTokenRequest
	if err := ctx.Bind(&request); err != nil {
		return response.Error(ctx, err)
	}

	if err := validator.Input(request); err != nil {
		return response.ErrorValidation(ctx, err)
	}

	user, err := h.service.User.GetTokens(ctx.Request().Context(), &request)
	if err != nil {
		return response.Error(ctx, err)
	}

	ExpiresAt := time.Now().Add(time.Second * time.Duration(user.Tokens[1].ExpiredIn))

	ctx.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    user.Tokens[1].Token,
		Expires:  ExpiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	return response.Success(ctx, http.StatusOK, user)
}

// GetUserByAccessToken retrieves user information by access token
// @Summary Get User By Access Token
// @Description Get user information by access token
// @Tags Users
// @Accept json
// @Produce json
// @Success 200 {object} models.Response{data=models.GetUserByAccountNumberResponse} "User Information Retrieved Successfully"
// @Failure 400 {object} models.Response "Invalid Input / Validation Error"
// @Failure 404 {object} models.Response "User Not Found"
// @Failure 500 {object} models.Response "Internal Server Error"
// @Router /api/v1/users/me [get]
// @Security BearerAuth
func (h *userV1Handler) GetUserByAccessToken(ctx echo.Context) error {
	accountNumber := ctx.Get("accountNumber").(string)

	user, err := h.service.User.GetByAccountNumber(ctx.Request().Context(), accountNumber)
	if err != nil {
		return response.Error(ctx, err)
	}

	return response.Success(ctx, http.StatusOK, user.GetUserByAccountNumberResponse())
}
