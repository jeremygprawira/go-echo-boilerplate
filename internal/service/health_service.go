package service

import (
	"context"
	"go-echo-boilerplate/internal/models"

	"github.com/jeremygprawira/wlog"
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
	postgreHealth := models.HealthDetailResponse{Type: models.TYPE_HEALTH, Component: "PostgreSQL", Status: "OK"}
	if err := hs.d.Repository.Health.Check(ctx); err != nil {
		// The driver error names the database role, database and host, and
		// /health is anonymous, so the detail goes to the wide event only.
		wlog.Set(ctx, "health.postgresql_error", err.Error())
		postgreHealth.Status = "ERROR"
		postgreHealth.Description = "PostgreSQL is unreachable"
	}

	healthy := postgreHealth.Status == "OK"
	description := "Service is healthy"
	if !healthy {
		description = "Service is unhealthy"
	}

	return &models.HealthResponse{
		Description:  description,
		Healthy:      healthy,
		Dependencies: []models.HealthDetailResponse{postgreHealth},
	}, nil
}
