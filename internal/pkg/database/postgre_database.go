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

func ConnectToPostgreSQL(cfg config.Validated) (*gorm.DB, error) {
	c := cfg.Get()
	connection := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s timezone=%s",
		c.PostgreSQL.Host,
		c.PostgreSQL.User,
		c.PostgreSQL.Password,
		c.PostgreSQL.Name,
		c.PostgreSQL.Port,
		c.PostgreSQL.SSLMode,
		c.Application.Timezone)

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  connection,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		Logger: gormLogger(c.Application.Environment, log.New(os.Stdout, "\r\n", log.LstdFlags)),
		NowFunc: func() time.Time {
			loc, err := time.LoadLocation(c.Application.Timezone)
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
	sqlDB.SetMaxIdleConns(c.PostgreSQL.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.PostgreSQL.MaxOpenConns)

	sqlDB.SetConnMaxLifetime(c.PostgreSQL.ConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// gormLogger silences GORM's built-in logger in production. Elsewhere it logs
// every statement at Info level, but with placeholders instead of bound
// values, so password hashes and PII never reach stdout.
func gormLogger(env config.Environment, w logger.Writer) logger.Interface {
	level := logger.Info
	if env.IsProduction() {
		level = logger.Silent
	}
	return logger.New(w, logger.Config{
		SlowThreshold:        200 * time.Millisecond,
		LogLevel:             level,
		ParameterizedQueries: true,
		Colorful:             true,
	})
}
