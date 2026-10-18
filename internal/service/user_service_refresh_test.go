package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type memRevoked struct{ rows map[string]time.Time }

func newMemRevoked() *memRevoked { return &memRevoked{rows: map[string]time.Time{}} }

func (m *memRevoked) RevokeToken(_ context.Context, jti string, expiresAt time.Time) (bool, error) {
	if _, ok := m.rows[jti]; ok {
		return false, nil
	}
	m.rows[jti] = expiresAt
	return true, nil
}

func (m *memRevoked) IsTokenRevoked(_ context.Context, jti string, now time.Time) (bool, error) {
	exp, ok := m.rows[jti]
	return ok && exp.After(now), nil
}

func (m *memRevoked) PurgeExpiredTokens(context.Context, time.Time) error { return nil }

func TestUserService_RefreshTokens(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := jwtc.SignRefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		mockRepo.On("UserByID", mock.Anything, 1).Return(user, nil)

		svc := service.NewUserService(fakeStore{mockRepo, newMemRevoked()}, testJWTConfig())

		resp, err := svc.RefreshTokens(context.Background(), refreshToken.Token)

		assert.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Tokens, 2)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Revoked refresh token is rejected", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := jwtc.SignRefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		mockRepo.On("UserByID", mock.Anything, 1).Return(user, nil)
		store := newMemRevoked()

		claims, err := jwtc.ParseRefreshToken(refreshToken.Token, testJWTConfig())
		require.NoError(t, err)
		_, err = store.RevokeToken(context.Background(), claims.ID, time.Now().Add(testJWTConfig().RefreshTokenDuration))
		require.NoError(t, err)

		svc := service.NewUserService(fakeStore{mockRepo, store}, testJWTConfig())

		resp, err := svc.RefreshTokens(context.Background(), refreshToken.Token)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errorc.Unauthorized.Is(err))
	})

	t.Run("Invalid refresh token is rejected", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		svc := service.NewUserService(fakeStore{mockRepo, newMemRevoked()}, testJWTConfig())

		resp, err := svc.RefreshTokens(context.Background(), "not-a-real-token")

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errorc.Unauthorized.Is(err))
	})

	t.Run("Deleted user is rejected, not issued fresh tokens", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := jwtc.SignRefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		mockRepo.On("UserByID", mock.Anything, 1).Return(nil, models.ErrNotFound)

		svc := service.NewUserService(fakeStore{mockRepo, newMemRevoked()}, testJWTConfig())

		resp, err := svc.RefreshTokens(context.Background(), refreshToken.Token)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.True(t, errorc.Unauthorized.Is(err))

		mockRepo.AssertExpectations(t)
	})

	t.Run("A failed user lookup is a database error and does not spend the token", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := jwtc.SignRefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		mockRepo.On("UserByID", mock.Anything, 1).Return(nil, errors.New("connection refused")).Once()
		mockRepo.On("UserByID", mock.Anything, 1).Return(user, nil).Once()

		svc := service.NewUserService(fakeStore{mockRepo, newMemRevoked()}, testJWTConfig())

		resp, err := svc.RefreshTokens(context.Background(), refreshToken.Token)
		assert.Nil(t, resp)
		assert.True(t, errorc.Database.Is(err), "got %v", err)

		resp, err = svc.RefreshTokens(context.Background(), refreshToken.Token)
		require.NoError(t, err, "the retry after the outage refreshes")
		assert.Len(t, resp.Tokens, 2)
	})
}

func TestUserService_Logout(t *testing.T) {
	t.Run("Revokes the access JTI", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		store := newMemRevoked()

		svc := service.NewUserService(fakeStore{mockRepo, store}, testJWTConfig())

		require.NoError(t, svc.Logout(context.Background(), "some-jti", ""))

		revoked, err := store.IsTokenRevoked(context.Background(), "some-jti", time.Now())
		require.NoError(t, err)
		assert.True(t, revoked)
	})

	t.Run("Also revokes the presented refresh token's JTI", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		store := newMemRevoked()
		user := &models.User{ID: 1, AccountNumber: "123456"}

		refreshToken, err := jwtc.SignRefreshToken(user, testJWTConfig())
		require.NoError(t, err)

		claims, err := jwtc.ParseRefreshToken(refreshToken.Token, testJWTConfig())
		require.NoError(t, err)

		svc := service.NewUserService(fakeStore{mockRepo, store}, testJWTConfig())

		require.NoError(t, svc.Logout(context.Background(), "some-jti", refreshToken.Token))

		accessRevoked, err := store.IsTokenRevoked(context.Background(), "some-jti", time.Now())
		require.NoError(t, err)
		assert.True(t, accessRevoked)

		refreshRevoked, err := store.IsTokenRevoked(context.Background(), claims.ID, time.Now())
		require.NoError(t, err)
		assert.True(t, refreshRevoked)
	})

	t.Run("An unparseable refresh token does not fail logout", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		store := newMemRevoked()

		svc := service.NewUserService(fakeStore{mockRepo, store}, testJWTConfig())

		require.NoError(t, svc.Logout(context.Background(), "some-jti", "not-a-real-token"))

		revoked, err := store.IsTokenRevoked(context.Background(), "some-jti", time.Now())
		require.NoError(t, err)
		assert.True(t, revoked)
	})
}
