//go:build postgres

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// postgresStore is the production-shaped Store. It is only compiled in
// with `go build -tags postgres`, which is what k8s/user-service.yaml's
// image is built with — see the Dockerfile.
type postgresStore struct {
	db *sql.DB
}

func newPostgresStore(dsn string) (Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	var lastErr error
	for i := 0; i < 10; i++ {
		if lastErr = db.Ping(); lastErr == nil {
			break
		}
		log.Printf("waiting for database (attempt %d/10): %v", i+1, lastErr)
		time.Sleep(2 * time.Second)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("ping db: %w", lastErr)
	}

	s := &postgresStore{db: db}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id         BIGSERIAL PRIMARY KEY,
			name       TEXT NOT NULL,
			age        INTEGER NOT NULL,
			location   TEXT NOT NULL,
			interests  TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *postgresStore) CreateUser(u User) (User, error) {
	row := s.db.QueryRow(
		`INSERT INTO users (name, age, location, interests) VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		u.Name, u.Age, u.Location, strings.Join(u.Interests, ","),
	)
	var createdAt time.Time
	if err := row.Scan(&u.ID, &createdAt); err != nil {
		return User{}, err
	}
	u.CreatedAt = createdAt.Format(time.RFC3339)
	return u, nil
}

func (s *postgresStore) GetUser(id int64) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, name, age, location, interests, created_at FROM users WHERE id = $1`, id,
	)
	u, err := scanUserRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func scanUserRow(row *sql.Row) (User, error) {
	var u User
	var interests string
	var createdAt time.Time
	if err := row.Scan(&u.ID, &u.Name, &u.Age, &u.Location, &interests, &createdAt); err != nil {
		return User{}, err
	}
	if interests != "" {
		u.Interests = strings.Split(interests, ",")
	}
	u.CreatedAt = createdAt.Format(time.RFC3339)
	return u, nil
}

func (s *postgresStore) ListUsers(excludeID int64) ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, name, age, location, interests, created_at FROM users WHERE id != $1 ORDER BY id`, excludeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var interests string
		var createdAt time.Time
		if err := rows.Scan(&u.ID, &u.Name, &u.Age, &u.Location, &interests, &createdAt); err != nil {
			return nil, err
		}
		if interests != "" {
			u.Interests = strings.Split(interests, ",")
		}
		u.CreatedAt = createdAt.Format(time.RFC3339)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *postgresStore) Healthy() error {
	return s.db.Ping()
}
