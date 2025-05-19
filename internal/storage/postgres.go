package storage

import (
    "database/sql"
    _ "github.com/lib/pq"
)

type PostgresStore struct {
    db *sql.DB
}

func NewPostgresStore(connStr string) (*PostgresStore, error) {
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, err
    }
    return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Migrate() error {
    _, err := s.db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id TEXT PRIMARY KEY,
            role TEXT NOT NULL,
            quota BIGINT NOT NULL
        )`)
    return err
}
