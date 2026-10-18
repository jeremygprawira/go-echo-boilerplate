package database

import (
	"go-echo-boilerplate/internal/config"

	"gorm.io/gorm"
)

type Database struct {
	PostgreDatabase *gorm.DB
}

// Connect to database, in case in future we will use another database, we can just add the Connect function
func Connect(cfg config.Validated) (*Database, error) {
	postgreDatabase, err := ConnectToPostgreSQL(cfg)
	if err != nil {
		return nil, err
	}

	return &Database{
		PostgreDatabase: postgreDatabase,
	}, nil
}
