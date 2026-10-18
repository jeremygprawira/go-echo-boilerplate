package database

import (
	"fmt"
	"go-echo-boilerplate/internal/config"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ConnectToPostgreSQL(config *config.Configuration) (*gorm.DB, error) {
	connection := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s timezone=%s",
		config.PostgreSQL.Host,
		config.PostgreSQL.User,
		config.PostgreSQL.Password,
		config.PostgreSQL.Name,
		config.PostgreSQL.Port,
		config.PostgreSQL.SSLMode,
		config.Application.Timezone)

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  connection,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		Logger: gormLogger(config, log.New(os.Stdout, "\r\n", log.LstdFlags)),
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

// gormLogger silences GORM's built-in logger in production. Elsewhere it logs
// every statement at Info level, but with placeholders instead of bound
// values, so password hashes and PII never reach stdout.
func gormLogger(config *config.Configuration, w logger.Writer) logger.Interface {
	level := logger.Info
	if config.IsProduction() {
		level = logger.Silent
	}
	return logger.New(w, logger.Config{
		SlowThreshold:        200 * time.Millisecond,
		LogLevel:             level,
		ParameterizedQueries: true,
		Colorful:             true,
	})
}
