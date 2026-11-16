// Command user-service owns user profile data for MicroHub.
//
// Persistence goes through the Store interface (store.go). By default it
// runs with an in-memory store (zero dependencies). Set DATABASE_URL and
// build with `-tags postgres` to run the production-shaped Postgres-backed
// store instead — see README.md and k8s/user-service.yaml.
package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// User is the profile record owned by this service.
type User struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Age       int      `json:"age"`
	Location  string   `json:"location"`
	Interests []string `json:"interests"`
	CreatedAt string   `json:"createdAt"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func main() {
	port := envOr("PORT", "8081")

	var store Store
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		s, err := newPostgresStore(dsn)
		if err != nil {
			log.Fatalf("startup: %v", err)
		}
		store = s
		log.Printf("user-service: using Postgres-backed store")
	} else {
		store = newMemoryStore()
		log.Printf("user-service: using in-memory store (set DATABASE_URL + build -tags postgres for production)")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Healthy(); err != nil {
			writeErr(w, http.StatusServiceUnavailable, "store unreachable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if u.Name == "" {
			writeErr(w, http.StatusBadRequest, "name is required")
			return
		}
		created, err := store.CreateUser(u)
		if err != nil {
			log.Printf("createUser: %v", err)
			writeErr(w, http.StatusInternalServerError, "could not create user")
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		u, err := store.GetUser(id)
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			log.Printf("getUser: %v", err)
			writeErr(w, http.StatusInternalServerError, "could not fetch user")
			return
		}
		writeJSON(w, http.StatusOK, u)
	})

	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		var excludeID int64
		if v := r.URL.Query().Get("exclude"); v != "" {
			excludeID, _ = strconv.ParseInt(v, 10, 64)
		}
		users, err := store.ListUsers(excludeID)
		if err != nil {
			log.Printf("listUsers: %v", err)
			writeErr(w, http.StatusInternalServerError, "could not list users")
			return
		}
		writeJSON(w, http.StatusOK, users)
	})

	log.Printf("user-service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, logRequests(mux)); err != nil {
		log.Fatal(err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
