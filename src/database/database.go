package database

import (
	"digitalUniversity/config"

	"github.com/jmoiron/sqlx"
)

func OpenDB(cfg *config.DatabaseConfig) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", cfg.URI)
	if err != nil {
		return nil, err
	}

	return db, nil
}
