package pgsql

import (
	"gorm.io/gorm"
)

type PostgreRepository struct {
	Health HealthRepository

	User UserRepository
}

func New(db *gorm.DB) *PostgreRepository {
	return &PostgreRepository{
		Health: NewHealthRepository(db),
		User:   NewUserRepository(db),
	}
}
