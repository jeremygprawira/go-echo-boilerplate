package service_test

import (
	"context"
	"testing"
	"time"

	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/tokenstore"
	"go-echo-boilerplate/internal/pkg/validator"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// inMemoryStore is a minimal TokenStore fake for exercising revocation logic
// without a real backend.
type inMemoryStore struct{ revoked map[string]bool }

func newInMemoryStore() *inMemoryStore { return &inMemoryStore{revoked: map[string]bool{}} }

func (s *inMemoryStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	s.revoked[jti] = true
	return nil
}

func (s *inMemoryStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return s.revoked[jti], nil
}

func TestUserService_RefreshTokens(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := generator.RefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		mockRepo.On("GetOneByID", mock.Anything, 1).Return(user, nil)

		deps := service.Dependencies{
			Repository: repository.Repository{User: mockRepo},
			JWTConfig:  testJWTConfig(),
			TokenStore: tokenstore.NewNoopStore(),
		}

		svc := service.NewUserService(&deps)

		resp, err := svc.RefreshTokens(context.Background(), refreshToken.Token)

		assert.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Tokens, 2)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Revoked refresh token is rejected", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := generator.RefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		store := newInMemoryStore()

		claims, err := validator.RefreshToken(refreshToken.Token, testJWTConfig())
		require.NoError(t, err)
		require.NoError(t, store.Revoke(context.Background(), claims.ID, testJWTConfig().RefreshTokenDuration))

		deps := service.Dependencies{
			Repository: repository.Repository{User: mockRepo},
			JWTConfig:  testJWTConfig(),
			TokenStore: store,
		}

		svc := service.NewUserService(&deps)

		resp, err := svc.RefreshTokens(context.Background(), refreshToken.Token)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errorc.Unauthorized.Is(err))
	})

	t.Run("Invalid refresh token is rejected", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		deps := service.Dependencies{
			Repository: repository.Repository{User: mockRepo},
			JWTConfig:  testJWTConfig(),
			TokenStore: tokenstore.NewNoopStore(),
		}

		svc := service.NewUserService(&deps)

		resp, err := svc.RefreshTokens(context.Background(), "not-a-real-token")

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errorc.Unauthorized.Is(err))
	})

	t.Run("Deleted user is rejected, not issued fresh tokens", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := generator.RefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		// GetOneByID returning (nil, nil) is how the repository reports "no such
		// user" (see pgsql.userRepository.GetOneByID) — RefreshTokens must treat
		// that as unauthorized rather than proceeding with a nil/zero-value user.
		mockRepo.On("GetOneByID", mock.Anything, 1).Return(nil, nil)

		deps := service.Dependencies{
			Repository: repository.Repository{User: mockRepo},
			JWTConfig:  testJWTConfig(),
			TokenStore: tokenstore.NewNoopStore(),
		}

		svc := service.NewUserService(&deps)

		resp, err := svc.RefreshTokens(context.Background(), refreshToken.Token)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errorc.Unauthorized.Is(err))

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_Logout(t *testing.T) {
	t.Run("Revokes the access JTI", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		store := newInMemoryStore()

		deps := service.Dependencies{
			Repository: repository.Repository{User: mockRepo},
			JWTConfig:  testJWTConfig(),
			TokenStore: store,
		}

		svc := service.NewUserService(&deps)

		require.NoError(t, svc.Logout(context.Background(), "some-jti", ""))

		revoked, err := store.IsRevoked(context.Background(), "some-jti")
		require.NoError(t, err)
		assert.True(t, revoked)
	})

	t.Run("Also revokes the presented refresh token's JTI", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		store := newInMemoryStore()
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := generator.RefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		claims, err := validator.RefreshToken(refreshToken.Token, testJWTConfig())
		require.NoError(t, err)

		deps := service.Dependencies{
			Repository: repository.Repository{User: mockRepo},
			JWTConfig:  testJWTConfig(),
			TokenStore: store,
		}

		svc := service.NewUserService(&deps)

		require.NoError(t, svc.Logout(context.Background(), "some-jti", refreshToken.Token))

		accessRevoked, err := store.IsRevoked(context.Background(), "some-jti")
		require.NoError(t, err)
		assert.True(t, accessRevoked)

		refreshRevoked, err := store.IsRevoked(context.Background(), claims.ID)
		require.NoError(t, err)
		assert.True(t, refreshRevoked)
	})

	t.Run("An unparseable refresh token does not fail logout", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		store := newInMemoryStore()

		deps := service.Dependencies{
			Repository: repository.Repository{User: mockRepo},
			JWTConfig:  testJWTConfig(),
			TokenStore: store,
		}

		svc := service.NewUserService(&deps)

		require.NoError(t, svc.Logout(context.Background(), "some-jti", "not-a-real-token"))

		revoked, err := store.IsRevoked(context.Background(), "some-jti")
		require.NoError(t, err)
		assert.True(t, revoked)
	})
}
