package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	httpdelivery "go-echo-boilerplate/internal/deliveries/http"
	v1 "go-echo-boilerplate/internal/deliveries/http/api/v1"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeremygprawira/herr"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// notRevoked is a RevocationChecker that reports every token as live.
type notRevoked struct{}

func (notRevoked) IsRevoked(context.Context, string) (bool, error) { return false, nil }

func strPtr(s string) *string {
	return &s
}

func newEcho() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	return e
}

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) Create(ctx context.Context, request *models.CreateUserRequest) (*models.User, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserService) GetTokens(ctx context.Context, request *models.GetUserTokenRequest) (*models.GetUserTokenResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GetUserTokenResponse), args.Error(1)
}

func (m *MockUserService) GetByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error) {
	args := m.Called(ctx, accountNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserService) RefreshTokens(ctx context.Context, refreshToken string) (*models.GetUserTokenResponse, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GetUserTokenResponse), args.Error(1)
}

func (m *MockUserService) Logout(ctx context.Context, accessJTI string, refreshToken string) error {
	args := m.Called(ctx, accessJTI, refreshToken)
	return args.Error(0)
}

func TestUserV1Handler_Create(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		e := newEcho()
		reqBody := models.CreateUserRequest{
			Name:     "Test User",
			Email:    "test@example.com",
			Password: "password123",
			PhoneNumber: models.PhoneNumber{
				Number:      "6281234567890",
				CountryCode: "ID",
			},
		}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("Create", mock.Anything, mock.MatchedBy(func(r *models.CreateUserRequest) bool {
			return r.Email == reqBody.Email
		})).Return(&models.User{
			ID:    1,
			Email: strPtr(reqBody.Email),
			Name:  reqBody.Name,
		}, nil)

		svc := &service.Service{
			User: mockSvc,
		}

		// Since NewUserV1 registers routes, we can't test handler method directly nicely unless we export it or use the router.
		// Use reflect or just checking the registered route?
		// Actually, standard way is to create the handler struct manually or call the constructor and then rely on Echo router matching.
		// However, NewUserV1 takes a Group.
		// Let's create a wrapper to assert the handler logic.
		// But wait, the handler struct `userV1Handler` is private.
		// So we must test via route dispatch or refactor to make handler testable.
		// Given I cannot easily refactor to export handler, testing via router is best.

		// Setup Group
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		// ServeRequest
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "Request has been successfully processed.")

		mockSvc.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		e := newEcho()
		reqBody := models.CreateUserRequest{
			// Missing required fields
		}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		// Setup Service
		svc := &service.Service{User: new(MockUserService)}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), `"errorCode":"VALIDATION_FAILED"`)
		assert.Contains(t, rec.Body.String(), `"errors":[`)
	})

	t.Run("Service Error - Already Exist", func(t *testing.T) {
		e := newEcho()
		reqBody := models.CreateUserRequest{
			Name:     "Test",
			Email:    "exist@example.com",
			Password: "pass",
		}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("Create", mock.Anything, mock.Anything).Return(nil, errorc.AlreadyExists.New())

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), `"errorCode":"ALREADY_EXISTS"`)
	})

	t.Run("Service Error - Internal Detail Never Leaks", func(t *testing.T) {
		e := newEcho()
		reqBody := models.CreateUserRequest{
			Name:     "Test",
			Email:    "test@example.com",
			Password: "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("Create", mock.Anything, mock.Anything).
			Return(nil, errorc.Database.New().Internal("failed to create user").Wrap(errors.New("pq: connection refused")))

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), `"errorCode":"DATABASE_ERROR"`)
		assert.NotContains(t, rec.Body.String(), "pq:")
		assert.NotContains(t, rec.Body.String(), "failed to create user")
	})
}

func TestUserV1Handler_GetTokens(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		e := newEcho()
		reqBody := models.GetUserTokenRequest{
			Email:    "test@example.com",
			Password: "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("GetTokens", mock.Anything, mock.MatchedBy(func(r *models.GetUserTokenRequest) bool {
			return r.Email == reqBody.Email
		})).Return(&models.GetUserTokenResponse{
			Email: reqBody.Email,
			Tokens: []models.Token{
				{Type: "access", Token: "access_token_mock"},
				{Type: "refresh", Token: "refresh_token_mock"},
			},
		}, nil)

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Request has been successfully processed.")
		assert.Contains(t, rec.Body.String(), "access_token_mock")

		mockSvc.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		e := newEcho()
		reqBody := models.GetUserTokenRequest{} // Empty request
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), `"errorCode":"VALIDATION_FAILED"`)
		assert.Contains(t, rec.Body.String(), `"errors":[`)
	})

	t.Run("Invalid Credentials", func(t *testing.T) {
		e := newEcho()
		reqBody := models.GetUserTokenRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("GetTokens", mock.Anything, mock.Anything).Return(nil, errorc.Unauthorized.New().Public(herr.Message("invalid credentials")))

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		// GetTokens returns Unauthorized/401 for both "no such user" and "wrong
		// password" (login-enumeration hardening); this test only verifies the
		// handler passes the service's error through unchanged.
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), `"errorCode":"UNAUTHORIZED"`)
		assert.Contains(t, rec.Body.String(), "invalid credentials")
	})
}

// TestUserV1Handler_RefreshTokens_TokenResolution covers the body-or-cookie
// precedence shared by RefreshTokens and Logout (jwtc.ResolveRefreshToken):
// an explicit body value wins; otherwise the refresh_token cookie is used;
// with neither present, the request is rejected before reaching the service.
func TestUserV1Handler_RefreshTokens_TokenResolution(t *testing.T) {
	t.Run("Body token takes precedence over cookie", func(t *testing.T) {
		e := newEcho()
		jsonBody, _ := json.Marshal(models.RefreshTokenRequest{RefreshToken: "body-token"})
		req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens/refresh", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "cookie-token"})
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("RefreshTokens", mock.Anything, "body-token").
			Return(&models.GetUserTokenResponse{Tokens: []models.Token{{Type: "access"}, {Type: "refresh"}}}, nil)

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Falls back to cookie when body omits the token", func(t *testing.T) {
		e := newEcho()
		req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens/refresh", bytes.NewReader([]byte("{}")))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "cookie-token"})
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("RefreshTokens", mock.Anything, "cookie-token").
			Return(&models.GetUserTokenResponse{Tokens: []models.Token{{Type: "access"}, {Type: "refresh"}}}, nil)

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Rejects when neither body nor cookie carries a token", func(t *testing.T) {
		e := newEcho()
		req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens/refresh", bytes.NewReader([]byte("{}")))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), `"errorCode":"VALIDATION_FAILED"`)
		mockSvc.AssertNotCalled(t, "RefreshTokens", mock.Anything, mock.Anything)
	})
}

// TestUserV1Handler_Logout_TokenResolution mirrors the refresh-token
// resolution coverage above for Logout, which is bearer-authed and so needs
// a real access token to pass BearerAuthMiddleware.
func TestUserV1Handler_Logout_TokenResolution(t *testing.T) {
	cfg := &jwtc.Configuration{
		AccessTokenSecret:   "access-secret",
		RefreshTokenSecret:  "refresh-secret",
		AccessTokenDuration: 15 * time.Minute,
		Issuer:              "test",
	}
	accessToken, err := generator.AccessToken(&models.User{ID: 1, AccountNumber: "1"}, cfg)
	require.NoError(t, err)

	t.Run("Body token takes precedence over cookie", func(t *testing.T) {
		e := newEcho()
		jsonBody, _ := json.Marshal(models.LogoutRequest{RefreshToken: "body-token"})
		req := httptest.NewRequest(http.MethodPost, "/v1/users/logout", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+accessToken.Token)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "cookie-token"})
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("Logout", mock.Anything, mock.Anything, "body-token").Return(nil)

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, cfg, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		mockSvc.AssertExpectations(t)
	})

	t.Run("Falls back to cookie when body omits the token", func(t *testing.T) {
		e := newEcho()
		req := httptest.NewRequest(http.MethodPost, "/v1/users/logout", nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+accessToken.Token)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "cookie-token"})
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("Logout", mock.Anything, mock.Anything, "cookie-token").Return(nil)

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, cfg, notRevoked{})

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		mockSvc.AssertExpectations(t)
	})
}
