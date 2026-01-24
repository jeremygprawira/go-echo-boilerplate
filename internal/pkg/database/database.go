package database

import (
	"go-echo-boilerplate/internal/config"

	"gorm.io/gorm"
)

type Database struct {
	PostgreDatabase *gorm.DB
}

// Connect to database, in case in future we will use another database, we can just add the Connect function
func Connect(config *config.Configuration) (*Database, error) {
	postgreDatabase, err := ConnectToPostgreSQL(config)
	if err != nil {
		return nil, err
	}

	return &Database{
		PostgreDatabase: postgreDatabase,
	}, nil
}

func Disconnect(db *Database) error {
	return DisconnectFromPostgreSQL(db.PostgreDatabase)
}
