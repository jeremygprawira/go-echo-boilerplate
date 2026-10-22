package http

import (
	"context"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/response"
	"net/http"

	"github.com/labstack/echo/v4"
)

type healthService interface {
	Check(ctx context.Context) (*models.HealthResponse, error)
}

type healthHandler struct {
	health healthService
}

// Check godoc
// @Summary Check health status
// @Description Check the health status of the service and its dependencies
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} models.Response{data=[]models.HealthDetailResponse}
// @Failure 503 {object} models.Response{data=[]models.HealthDetailResponse} "A dependency is unreachable"
// @Router /health [get]
func (h healthHandler) Check(ctx echo.Context) error {
	health, err := h.health.Check(ctx.Request().Context())
	if err != nil {
		return err
	}

	code := http.StatusOK
	if !health.Healthy {
		code = http.StatusServiceUnavailable
	}
	return response.SuccessList(ctx, code, health.Description, health.Dependencies)
}
