package store

import (
	"database/sql"

	"github.com/nanobazaar/relay/internal/store/sqlc"
)

type Store struct {
	DB *sql.DB
	*sqlc.Queries
}

func New(db *sql.DB) *Store {
	return &Store{DB: db, Queries: sqlc.New(db)}
}
