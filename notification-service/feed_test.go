package main

import "testing"

func TestFeedStore_AddAndGet(t *testing.T) {
	s := newFeedStore()
	s.add(1, "hello")
	s.add(1, "world")
	s.add(2, "other user")

	feed1 := s.get(1)
	if len(feed1) != 2 {
		t.Fatalf("expected 2 notifications for user 1, got %d", len(feed1))
	}
	if feed1[0].Message != "hello" || feed1[1].Message != "world" {
		t.Fatalf("notifications out of order or wrong content: %+v", feed1)
	}

	feed2 := s.get(2)
	if len(feed2) != 1 || feed2[0].Message != "other user" {
		t.Fatalf("expected 1 notification for user 2, got %+v", feed2)
	}
}

func TestFeedStore_UnknownUserReturnsEmpty(t *testing.T) {
	s := newFeedStore()
	if feed := s.get(42); len(feed) != 0 {
		t.Fatalf("expected empty feed for unknown user, got %+v", feed)
	}
}
