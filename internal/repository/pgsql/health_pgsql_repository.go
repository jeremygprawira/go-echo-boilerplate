package pgsql

import (
	"context"

	"gorm.io/gorm"
)

type HealthRepository interface {
	Check(ctx context.Context) error
}

type healthRepository struct {
	db *gorm.DB
}

func NewHealthRepository(db *gorm.DB) HealthRepository {
	return &healthRepository{db: db}
}

func (hr *healthRepository) Check(ctx context.Context) error {
	pgsql, err := hr.db.DB()
	if err != nil {
		return err
	}

	return pgsql.PingContext(ctx)
}
