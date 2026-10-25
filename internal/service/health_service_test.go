package service_test

import (
	"context"
	"errors"
	"go-echo-boilerplate/internal/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHealthRepository
type MockHealthRepository struct {
	mock.Mock
}

func (m *MockHealthRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestHealthService_Check(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockHealthRepository)
		mockRepo.On("Ping", mock.Anything).Return(nil)
		svc := service.NewHealthService(mockRepo)
		resp, err := svc.Check(context.Background())

		assert.NoError(t, err)
		assert.Equal(t, "Service is healthy", resp.Description)
		assert.True(t, resp.Healthy)
		assert.Len(t, resp.Dependencies, 1) // Expecting Postgre result
		assert.Equal(t, "OK", resp.Dependencies[0].Status)

		mockRepo.AssertExpectations(t)
	})

	t.Run("PostgreSQL Failure", func(t *testing.T) {
		mockRepo := new(MockHealthRepository)
		mockRepo.On("Ping", mock.Anything).Return(errors.New("connection refused"))

		svc := service.NewHealthService(mockRepo)
		resp, err := svc.Check(context.Background())

		assert.NoError(t, err)
		assert.Equal(t, "Service is unhealthy", resp.Description)
		assert.False(t, resp.Healthy)
		assert.Len(t, resp.Dependencies, 1)
		assert.Equal(t, "ERROR", resp.Dependencies[0].Status)
		assert.Equal(t, "PostgreSQL is unreachable", resp.Dependencies[0].Description)

		mockRepo.AssertExpectations(t)
	})
}
