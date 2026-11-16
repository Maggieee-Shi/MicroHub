package main

import "errors"

// ErrNotFound is returned by a Store when a user id does not exist.
var ErrNotFound = errors.New("user not found")

// Store is the persistence interface user-service's HTTP handlers depend
// on. Keeping it as an interface (rather than calling *sql.DB directly from
// the handlers) is what lets this service run two ways from the same
// handler code:
//
//   - memoryStore: zero dependencies, used by default. Good for local
//     development, tests, and CI that doesn't want to stand up a database.
//   - postgresStore: the production-shaped implementation backed by
//     Postgres, built in with `go build -tags postgres` (see README). This
//     is the implementation the Kubernetes manifests in k8s/ assume.
type Store interface {
	CreateUser(u User) (User, error)
	GetUser(id int64) (User, error)
	ListUsers(excludeID int64) ([]User, error)
	Healthy() error
}
