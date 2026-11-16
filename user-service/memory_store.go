package main

import (
	"sync"
	"time"
)

// memoryStore is a thread-safe, in-memory Store. It is the default so the
// service has zero third-party dependencies and `go build` / `go run` work
// immediately on any machine with no network access and no database.
type memoryStore struct {
	mu     sync.Mutex
	nextID int64
	users  map[int64]User
}

func newMemoryStore() Store {
	return &memoryStore{nextID: 1, users: make(map[int64]User)}
}

func (s *memoryStore) CreateUser(u User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u.ID = s.nextID
	s.nextID++
	u.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	s.users[u.ID] = u
	return u, nil
}

func (s *memoryStore) GetUser(id int64) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (s *memoryStore) ListUsers(excludeID int64) ([]User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []User
	for id, u := range s.users {
		if id == excludeID {
			continue
		}
		out = append(out, u)
	}
	return out, nil
}

func (s *memoryStore) Healthy() error { return nil }
