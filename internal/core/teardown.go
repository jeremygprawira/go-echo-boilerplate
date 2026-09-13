package core

import (
	"context"
	"database/sql"
)

var db *sql.DB

func setDB(database *sql.DB) {
	db = database
}

func Teardown(ctx context.Context) error {
	if db != nil {
		return db.Close()
	}
	return nil
}
