package core

import (
	"context"
	"database/sql"
)

var db *sql.DB

func setDB(database *sql.DB) {
	db = database
}

var infraClients interface{ Close() error }

func setClients(c interface{ Close() error }) {
	infraClients = c
}

func Teardown(ctx context.Context) error {
	if infraClients != nil {
		if err := infraClients.Close(); err != nil {
			return err
		}
	}
	if db != nil {
		return db.Close()
	}
	return nil
}
