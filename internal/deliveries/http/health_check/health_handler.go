package healthcheck

import (
	"go-echo-boilerplate/internal/pkg/response"
	"go-echo-boilerplate/internal/service"

	"net/http"

	"github.com/labstack/echo/v4"
)

type healthHandler struct {
	service *service.Service
}

func New(api *echo.Group, service *service.Service) {
	hc := healthHandler{
		service: service,
	}

	api.GET("", hc.Check)
}

// Check godoc
// @Summary Check health status
// @Description Check the health status of the service and its dependencies
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} models.Response{data=[]models.HealthDetailResponse}
// @Failure 500 {object} models.ErrorResponse
// @Router /health [get]
func (h *healthHandler) Check(ctx echo.Context) error {
	health, err := h.service.Health.Check(ctx.Request().Context())
	if err != nil {
		return response.Error(ctx, err)
	}

	return response.SuccessList(ctx, http.StatusOK, health.Description, health.Dependencies)
}
