package service

import (
	"context"
	"fmt"
	"go-echo-boilerplate/internal/models"
)

type HealthService interface {
	Check(ctx context.Context) (*models.HealthResponse, error)
}

type healthService struct {
	d *Dependencies
}

func NewHealthService(d *Dependencies) HealthService {
	return &healthService{d: d}
}

func (hs *healthService) Check(ctx context.Context) (*models.HealthResponse, error) {
	var healthDetail []models.HealthDetailResponse

	var postgreHealth models.HealthDetailResponse
	postgreErr := hs.d.Repository.Postgre.Health.Check(ctx)
	if postgreErr != nil {
		postgreHealth = models.HealthDetailResponse{
			Type:        models.TYPE_HEALTH,
			Component:   "PostgreSQL",
			Status:      "ERROR",
			Description: fmt.Sprintf("PostgreSQL is not healthy, due to %v", postgreErr),
		}

		healthDetail = append(healthDetail, postgreHealth)
	} else {
		postgreHealth = models.HealthDetailResponse{
			Type:      models.TYPE_HEALTH,
			Component: "PostgreSQL",
			Status:    "OK",
		}

		healthDetail = append(healthDetail, postgreHealth)
	}

	return &models.HealthResponse{
		Description:  "Service is healthy",
		Dependencies: healthDetail,
	}, nil
}
