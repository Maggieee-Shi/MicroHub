// Command matching-service is the public-facing API in MicroHub.
//
// It owns swipe state and match detection, and talks to user-service and
// notification-service over plain HTTP using Kubernetes' built-in DNS-based
// service discovery (each dependency is addressed by its Service name, e.g.
// "http://user-service:8081", rather than a hardcoded IP).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type swipe struct {
	Liked bool
}

// swipeStore is an in-memory, mutex-guarded record of who has swiped on
// whom. A real system would persist this, but for MicroHub's scope an
// in-memory store keeps the service simple while still proving out the
// concurrency-safety and inter-service-call pieces the project is meant to
// demonstrate.
type swipeStore struct {
	mu     sync.Mutex
	swipes map[int64]map[int64]swipe // userID -> targetID -> swipe
	matches map[int64]map[int64]bool // userID -> set of matched userIDs
}

func newSwipeStore() *swipeStore {
	return &swipeStore{
		swipes:  make(map[int64]map[int64]swipe),
		matches: make(map[int64]map[int64]bool),
	}
}

// recordSwipe stores userID's swipe on targetID and reports whether this
// swipe just completed a mutual match.
func (s *swipeStore) recordSwipe(userID, targetID int64, liked bool) (isNewMatch bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.swipes[userID] == nil {
		s.swipes[userID] = make(map[int64]swipe)
	}
	s.swipes[userID][targetID] = swipe{Liked: liked}

	if !liked {
		return false
	}

	reciprocated := s.swipes[targetID] != nil && s.swipes[targetID][userID].Liked
	if !reciprocated {
		return false
	}

	if s.matches[userID] == nil {
		s.matches[userID] = make(map[int64]bool)
	}
	if s.matches[targetID] == nil {
		s.matches[targetID] = make(map[int64]bool)
	}
	alreadyMatched := s.matches[userID][targetID]
	s.matches[userID][targetID] = true
	s.matches[targetID][userID] = true
	return !alreadyMatched
}

func (s *swipeStore) matchesFor(userID int64) []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []int64
	for id := range s.matches[userID] {
		out = append(out, id)
	}
	return out
}

type userClient struct {
	baseURL string
	client  *http.Client
}

type userProfile struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (c *userClient) fetchUser(id int64) (userProfile, error) {
	resp, err := c.client.Get(fmt.Sprintf("%s/users/%d", c.baseURL, id))
	if err != nil {
		return userProfile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return userProfile{}, fmt.Errorf("user-service returned %d: %s", resp.StatusCode, body)
	}
	var u userProfile
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return userProfile{}, err
	}
	return u, nil
}

type notifier struct {
	baseURL string
	client  *http.Client
}

func (n *notifier) notify(userID int64, message string) error {
	payload, _ := json.Marshal(map[string]any{"userId": userID, "message": message})
	resp, err := n.client.Post(n.baseURL+"/notifications", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification-service returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type swipeRequest struct {
	UserID   int64 `json:"userId"`
	TargetID int64 `json:"targetId"`
	Liked    bool  `json:"liked"`
}

func main() {
	userServiceURL := envOr("USER_SERVICE_URL", "http://localhost:8081")
	notificationServiceURL := envOr("NOTIFICATION_SERVICE_URL", "http://localhost:8082")
	port := envOr("PORT", "8080")

	httpClient := &http.Client{Timeout: 5 * time.Second}
	users := &userClient{baseURL: userServiceURL, client: httpClient}
	notifications := &notifier{baseURL: notificationServiceURL, client: httpClient}
	store := newSwipeStore()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /swipes", func(w http.ResponseWriter, r *http.Request) {
		var req swipeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if req.UserID == 0 || req.TargetID == 0 {
			writeErr(w, http.StatusBadRequest, "userId and targetId are required")
			return
		}

		// Verify both users actually exist by calling user-service. This is
		// the inter-service call that exercises Kubernetes service
		// discovery end to end.
		if _, err := users.fetchUser(req.UserID); err != nil {
			log.Printf("fetchUser(%d): %v", req.UserID, err)
			writeErr(w, http.StatusBadRequest, "unknown userId")
			return
		}
		target, err := users.fetchUser(req.TargetID)
		if err != nil {
			log.Printf("fetchUser(%d): %v", req.TargetID, err)
			writeErr(w, http.StatusBadRequest, "unknown targetId")
			return
		}

		isNewMatch := store.recordSwipe(req.UserID, req.TargetID, req.Liked)

		resp := map[string]any{"matched": isNewMatch}
		if isNewMatch {
			requester, _ := users.fetchUser(req.UserID)
			msg1 := fmt.Sprintf("You matched with %s!", target.Name)
			msg2 := fmt.Sprintf("You matched with %s!", requester.Name)
			if err := notifications.notify(req.UserID, msg1); err != nil {
				log.Printf("notify(%d): %v", req.UserID, err)
			}
			if err := notifications.notify(req.TargetID, msg2); err != nil {
				log.Printf("notify(%d): %v", req.TargetID, err)
			}
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("GET /matches/{userId}", func(w http.ResponseWriter, r *http.Request) {
		var userID int64
		fmt.Sscanf(r.PathValue("userId"), "%d", &userID)
		ids := store.matchesFor(userID)
		writeJSON(w, http.StatusOK, map[string]any{"userId": userID, "matchedWith": ids})
	})

	log.Printf("matching-service listening on :%s (user-service=%s, notification-service=%s)",
		port, userServiceURL, notificationServiceURL)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
