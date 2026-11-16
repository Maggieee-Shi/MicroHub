//go:build !postgres

package main

import "fmt"

// newPostgresStore's stub is compiled in for the default build (no
// "postgres" build tag), so main.go can always reference the function
// without the default, dependency-free build needing to know about
// github.com/lib/pq at all.
func newPostgresStore(dsn string) (Store, error) {
	return nil, fmt.Errorf("this binary was built without Postgres support; rebuild with: go build -tags postgres")
}
