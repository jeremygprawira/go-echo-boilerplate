package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	v1 "go-echo-boilerplate/internal/deliveries/http/api/v1"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func strPtr(s string) *string {
	return &s
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

func TestUserV1Handler_Create(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		e := echo.New()
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
		v1.NewUserV1(g, svc, nil, nil)

		// ServeRequest
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "Request has been successfully processed.")

		mockSvc.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		e := echo.New()
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
		v1.NewUserV1(g, svc, nil, nil)

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Validation failed")
	})

	t.Run("Service Error - Already Exist", func(t *testing.T) {
		e := echo.New()
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
		mockSvc.On("Create", mock.Anything, mock.Anything).Return(nil, errorc.Error(errorc.ErrorAlreadyExist))

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil)

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
	})
}

func TestUserV1Handler_GetTokens(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		e := echo.New()
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
		v1.NewUserV1(g, svc, nil, nil)

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Request has been successfully processed.")
		assert.Contains(t, rec.Body.String(), "access_token_mock")

		mockSvc.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		e := echo.New()
		reqBody := models.GetUserTokenRequest{} // Empty request
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil)

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Validation failed")
	})

	t.Run("Invalid Credentials", func(t *testing.T) {
		e := echo.New()
		reqBody := models.GetUserTokenRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("GetTokens", mock.Anything, mock.Anything).Return(nil, errorc.Error(errorc.ErrorInvalidInput))

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil)

		e.ServeHTTP(rec, req)

		// ErrorInvalidInput typically maps to 400 Bad Request
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
