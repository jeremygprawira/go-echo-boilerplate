package service

import (
	"context"
	"go-echo-boilerplate/internal/models"

	"github.com/jeremygprawira/wlog"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type HealthService struct {
	db pinger
}

func NewHealthService(db pinger) *HealthService {
	return &HealthService{db: db}
}

func (hs *HealthService) Check(ctx context.Context) (*models.HealthResponse, error) {
	postgreHealth := models.HealthDetailResponse{Type: models.TYPE_HEALTH, Component: "PostgreSQL", Status: "OK"}
	if err := hs.db.Ping(ctx); err != nil {
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
