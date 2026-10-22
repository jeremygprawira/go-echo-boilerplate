package http

import (
	"context"
	"encoding/json"
	"go-echo-boilerplate/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHealthService
type MockHealthService struct {
	mock.Mock
}

func (m *MockHealthService) Check(ctx context.Context) (*models.HealthResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.HealthResponse), args.Error(1)
}

func TestHealthHandler_Check(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		mockHealthSvc := new(MockHealthService)
		expectedResp := &models.HealthResponse{
			Description: "Service is healthy",
			Healthy:     true,
			Dependencies: []models.HealthDetailResponse{
				{Component: "PostgreSQL", Status: "OK"},
			},
		}
		mockHealthSvc.On("Check", mock.Anything).Return(expectedResp, nil)

		e := testServer{health: mockHealthSvc}.build(t)

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.NoError(t, err)

		// Assert top-level fields
		// Code might be float64 when unmarshaled to interface{}
		assert.Equal(t, float64(200), resp["code"])
		assert.Equal(t, "OK", resp["status"])
		assert.Equal(t, "Service is healthy", resp["message"])

		data, ok := resp["data"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, data, 1)
	})
}
