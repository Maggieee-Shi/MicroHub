// Command notification-service is a small fan-in service: it receives
// "a match happened" events from matching-service and keeps a per-user
// notification feed. Deliberately simple (in-memory, no external deps) so
// it is cheap to run everywhere, including as the smallest Docker image in
// the system.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Notification struct {
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
}

type feedStore struct {
	mu   sync.Mutex
	feed map[int64][]Notification
}

func newFeedStore() *feedStore {
	return &feedStore{feed: make(map[int64][]Notification)}
}

func (s *feedStore) add(userID int64, message string) Notification {
	n := Notification{Message: message, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feed[userID] = append(s.feed[userID], n)
	return n
}

func (s *feedStore) get(userID int64) []Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Notification, len(s.feed[userID]))
	copy(out, s.feed[userID])
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type createRequest struct {
	UserID  int64  `json:"userId"`
	Message string `json:"message"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	store := newFeedStore()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /notifications", func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if req.UserID == 0 || req.Message == "" {
			writeErr(w, http.StatusBadRequest, "userId and message are required")
			return
		}
		n := store.add(req.UserID, req.Message)
		log.Printf("notification queued for user %d: %s", req.UserID, req.Message)
		writeJSON(w, http.StatusCreated, n)
	})

	mux.HandleFunc("GET /notifications/{userId}", func(w http.ResponseWriter, r *http.Request) {
		var userID int64
		fmt.Sscanf(r.PathValue("userId"), "%d", &userID)
		writeJSON(w, http.StatusOK, store.get(userID))
	})

	log.Printf("notification-service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
