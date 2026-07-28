package service_test

import (
	"context"
	"errors"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/apperr"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/service"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testJWTConfig() *jwtc.Configuration {
	return &jwtc.Configuration{
		AccessTokenSecret:    "access-secret-aaaaaaaaaaaaaaaaaaaa",
		RefreshTokenSecret:   "refresh-secret-bbbbbbbbbbbbbbbbbbbb",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
		Issuer:               "test-issuer",
	}
}

func strPtr(s string) *string {
	return &s
}

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) CheckByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (bool, error) {
	args := m.Called(ctx, email, phoneNumber)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) GetCredentialsByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (*models.User, error) {
	args := m.Called(ctx, email, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetOneByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error) {
	args := m.Called(ctx, accountNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetOneByID(ctx context.Context, id int) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func TestUserService_Create(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *models.User) bool {
			return u.Email != nil && *u.Email == "test@example.com" && u.Name == "Test User"
		})).Return(nil)

		deps := service.Dependencies{
			Repository: repository.Repository{
				User: mockRepo,
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.CreateUserRequest{
			Name:  "Test User",
			Email: "test@example.com",
			PhoneNumber: models.PhoneNumber{
				Number:      "081234567890", // Will be formatted to 6281234567890
				CountryCode: "ID",
			},
			Password: "password123",
		}

		user, err := svc.Create(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		if assert.NotNil(t, user.Email) {
			assert.Equal(t, req.Email, *user.Email)
		}
		assert.NotEmpty(t, user.AccountNumber)
		assert.NotEqual(t, "password123", user.Password) // Should be hashed

		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid Phone Number", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		deps := service.Dependencies{
			Repository: repository.Repository{
				User: mockRepo,
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.CreateUserRequest{
			PhoneNumber: models.PhoneNumber{
				Number: "invalid",
			},
		}

		user, err := svc.Create(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.True(t, apperr.InvalidInput.Is(err))
	})

	t.Run("User Already Exists", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("Create", mock.Anything, mock.Anything).
			Return(&pgconn.PgError{Code: "23505"})

		deps := service.Dependencies{
			Repository: repository.Repository{
				User: mockRepo,
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.CreateUserRequest{
			Email:    "existing@example.com",
			Password: "password123",
		}

		user, err := svc.Create(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.True(t, apperr.AlreadyExists.Is(err))

		mockRepo.AssertExpectations(t)
	})

	t.Run("DB Create Error", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db create error"))

		deps := service.Dependencies{
			Repository: repository.Repository{
				User: mockRepo,
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.CreateUserRequest{
			Email:    "test@example.com",
			Password: "password123",
		}

		user, err := svc.Create(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.True(t, apperr.Database.Is(err))

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_GetTokens(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		hashedPassword, _ := generator.Hash("password123")

		mockRepo.On("GetCredentialsByEmailOrPhoneNumber", mock.Anything, "test@example.com", "").
			Return(&models.User{
				ID:            1,
				Email:         strPtr("test@example.com"),
				Password:      hashedPassword,
				Name:          "Test User",
				AccountNumber: "123456",
			}, nil)

		deps := service.Dependencies{
			Repository: repository.Repository{
				User: mockRepo,
			},
			JWTConfig: testJWTConfig(),
		}

		svc := service.NewUserService(&deps)

		req := &models.GetUserTokenRequest{
			Email:    "test@example.com",
			Password: "password123",
		}

		resp, err := svc.GetTokens(context.Background(), req)

		assert.NoError(t, err)
		if !assert.NotNil(t, resp) {
			t.FailNow()
		}
		assert.Equal(t, "test@example.com", resp.Email)
		assert.Len(t, resp.Tokens, 2) // Access and Refresh

		mockRepo.AssertExpectations(t)
	})

	// User-not-found and wrong-password now return the SAME generic 401 error
	// (see Task 6: remove login user-enumeration signal) so an attacker cannot
	// distinguish "no such account" from "wrong password" by message or timing.

	t.Run("User Not Found", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("GetCredentialsByEmailOrPhoneNumber", mock.Anything, "notfound@example.com", "").Return(nil, nil)

		deps := service.Dependencies{
			Repository: repository.Repository{
				User: mockRepo,
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.GetUserTokenRequest{
			Email:    "notfound@example.com",
			Password: "any",
		}

		resp, err := svc.GetTokens(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, apperr.Unauthorized.Is(err))

		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid Password", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		hashedPassword, _ := generator.Hash("password123")

		mockRepo.On("GetCredentialsByEmailOrPhoneNumber", mock.Anything, "test@example.com", "").
			Return(&models.User{
				ID:       1,
				Email:    strPtr("test@example.com"),
				Password: hashedPassword,
			}, nil)

		deps := service.Dependencies{
			Repository: repository.Repository{
				User: mockRepo,
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.GetUserTokenRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}

		resp, err := svc.GetTokens(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, apperr.Unauthorized.Is(err))

		mockRepo.AssertExpectations(t)
	})
}

func TestGetTokens_UserNotFound_GenericError(t *testing.T) {
	mockRepo := new(MockUserRepository)
	mockRepo.
		On("GetCredentialsByEmailOrPhoneNumber", mock.Anything, "nobody@example.com", "").
		Return((*models.User)(nil), nil)

	deps := service.Dependencies{
		Repository: repository.Repository{
			User: mockRepo,
		},
	}
	svc := service.NewUserService(&deps)

	_, err := svc.GetTokens(context.Background(), &models.GetUserTokenRequest{
		Email:    "nobody@example.com",
		Password: "whatever",
	})

	require.Error(t, err)
	require.True(t, apperr.Unauthorized.Is(err))

	mockRepo.AssertExpectations(t)
}

func TestGetTokens_WrongPassword_SameGenericError(t *testing.T) {
	hashed, _ := generator.Hash("correct-password")
	mockRepo := new(MockUserRepository)
	mockRepo.
		On("GetCredentialsByEmailOrPhoneNumber", mock.Anything, "user@example.com", "").
		Return(&models.User{ID: 1, AccountNumber: "1", Password: hashed}, nil)

	deps := service.Dependencies{
		Repository: repository.Repository{
			User: mockRepo,
		},
	}
	svc := service.NewUserService(&deps)

	_, err := svc.GetTokens(context.Background(), &models.GetUserTokenRequest{
		Email:    "user@example.com",
		Password: "wrong-password",
	})

	require.Error(t, err)
	require.True(t, apperr.Unauthorized.Is(err))

	mockRepo.AssertExpectations(t)
}
