package database

import (
	"fmt"
	"go-echo-boilerplate/internal/config"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectToPostgreSQL(config *config.Configuration) (*gorm.DB, error) {
	connection := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s timezone=Asia/Jakarta",
		config.PostgreSQL.Host,
		config.PostgreSQL.User,
		config.PostgreSQL.Password,
		config.PostgreSQL.Name,
		config.PostgreSQL.Port,
		config.PostgreSQL.SSLMode)

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  connection,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		NowFunc: func() time.Time {
			loc, err := time.LoadLocation(config.Application.Timezone)
			if err != nil {
				return time.Now() // Fallback to system default if location loading fails
			}
			return time.Now().In(loc)
		},
	})

	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying *sql.DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(config.PostgreSQL.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.PostgreSQL.MaxOpenConns)

	if config.PostgreSQL.ConnMaxLifetime != "" {
		lifetime, err := time.ParseDuration(config.PostgreSQL.ConnMaxLifetime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse conn_max_lifetime: %w", err)
		}
		sqlDB.SetConnMaxLifetime(lifetime)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func DisconnectFromPostgreSQL(db *gorm.DB) error {
	pg, err := db.DB()
	if err != nil {
		return err
	}

	return pg.Close()
}
