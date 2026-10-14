package pgsql

import (
	"gorm.io/gorm"
)

type PostgreRepository struct {
	Health HealthRepository

	User         UserRepository
	RevokedToken RevokedTokenRepository
}

func New(db *gorm.DB) *PostgreRepository {
	return &PostgreRepository{
		Health:       NewHealthRepository(db),
		User:         NewUserRepository(db),
		RevokedToken: NewRevokedTokenRepository(db),
	}
}
