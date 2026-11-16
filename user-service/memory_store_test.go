package main

import (
	"errors"
	"testing"
)

func TestMemoryStore_CreateAndGet(t *testing.T) {
	s := newMemoryStore()
	created, err := s.CreateUser(User{Name: "Maggie", Age: 24, Location: "SF", Interests: []string{"go", "hiking"}})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("expected a non-zero ID to be assigned")
	}
	if created.CreatedAt == "" {
		t.Fatalf("expected CreatedAt to be set")
	}

	got, err := s.GetUser(created.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Name != "Maggie" || len(got.Interests) != 2 {
		t.Fatalf("got unexpected user back: %+v", got)
	}
}

func TestMemoryStore_GetUnknownUserReturnsNotFound(t *testing.T) {
	s := newMemoryStore()
	_, err := s.GetUser(999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_ListUsersExcludesGivenID(t *testing.T) {
	s := newMemoryStore()
	a, _ := s.CreateUser(User{Name: "A"})
	_, _ = s.CreateUser(User{Name: "B"})

	list, err := s.ListUsers(a.ID)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(list) != 1 || list[0].Name != "B" {
		t.Fatalf("expected only B to be listed, got %+v", list)
	}
}
