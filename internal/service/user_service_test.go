package service_test

import (
	"context"
	"errors"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/repository/pgsql"
	"go-echo-boilerplate/internal/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

func TestUserService_Create(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		// Expect CheckByEmailOrPhoneNumber to return false (not found)
		mockRepo.On("CheckByEmailOrPhoneNumber", mock.Anything, "test@example.com", "+6281234567890").Return(false, nil)

		// Expect Create to succeed
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *models.User) bool {
			return u.Email != nil && *u.Email == "test@example.com" && u.Name == "Test User"
		})).Return(nil)

		deps := service.Dependencies{
			Repository: repository.Repository{
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
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
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
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
		assert.Contains(t, err.Error(), "Invalid phone number format") // Matches errorc.ErrorInvalidInput or message
		// Note check specific error message based on implementation
	})

	t.Run("User Already Exists", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("CheckByEmailOrPhoneNumber", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

		deps := service.Dependencies{
			Repository: repository.Repository{
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
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
		assert.Equal(t, errorc.ErrorAlreadyExist.Response.Code, errorc.GetResponse(err).Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("DB Check Error", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("CheckByEmailOrPhoneNumber", mock.Anything, mock.Anything, mock.Anything).Return(false, errors.New("db error"))

		deps := service.Dependencies{
			Repository: repository.Repository{
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.CreateUserRequest{
			Email: "test@example.com",
		}

		user, err := svc.Create(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, errorc.ErrorDatabase.Response.Code, errorc.GetResponse(err).Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("DB Create Error", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("CheckByEmailOrPhoneNumber", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db create error"))

		deps := service.Dependencies{
			Repository: repository.Repository{
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
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
		assert.Equal(t, errorc.ErrorDatabase.Response.Code, errorc.GetResponse(err).Code)

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
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
			},
			// JWTConfig is nil, rely on generator defaults
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

	t.Run("User Not Found", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("GetCredentialsByEmailOrPhoneNumber", mock.Anything, "notfound@example.com", "").Return(nil, nil)

		deps := service.Dependencies{
			Repository: repository.Repository{
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
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
		assert.Equal(t, errorc.ErrorDataNotFound.Response.Code, errorc.GetResponse(err).Code)

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
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
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
		assert.Equal(t, errorc.ErrorInvalidInput.Response.Code, errorc.GetResponse(err).Code)

		mockRepo.AssertExpectations(t)
	})
}
