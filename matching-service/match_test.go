package main

import "testing"

func TestRecordSwipe_NoMatchOnOneSidedLike(t *testing.T) {
	s := newSwipeStore()
	isMatch := s.recordSwipe(1, 2, true)
	if isMatch {
		t.Fatalf("one-sided like should not be a match")
	}
	if got := s.matchesFor(1); len(got) != 0 {
		t.Fatalf("expected no matches for user 1, got %v", got)
	}
}

func TestRecordSwipe_MutualLikeIsMatch(t *testing.T) {
	s := newSwipeStore()
	if isMatch := s.recordSwipe(1, 2, true); isMatch {
		t.Fatalf("first swipe should not yet be a match")
	}
	isMatch := s.recordSwipe(2, 1, true)
	if !isMatch {
		t.Fatalf("mutual like should produce a match")
	}

	got1 := s.matchesFor(1)
	got2 := s.matchesFor(2)
	if len(got1) != 1 || got1[0] != 2 {
		t.Fatalf("expected user 1 matched with [2], got %v", got1)
	}
	if len(got2) != 1 || got2[0] != 1 {
		t.Fatalf("expected user 2 matched with [1], got %v", got2)
	}
}

func TestRecordSwipe_DislikeNeverMatches(t *testing.T) {
	s := newSwipeStore()
	s.recordSwipe(1, 2, true)
	isMatch := s.recordSwipe(2, 1, false) // 2 passes on 1
	if isMatch {
		t.Fatalf("a dislike should never produce a match")
	}
}

func TestRecordSwipe_IsIdempotentAfterMatch(t *testing.T) {
	s := newSwipeStore()
	s.recordSwipe(1, 2, true)
	first := s.recordSwipe(2, 1, true)
	if !first {
		t.Fatalf("expected the second mutual swipe to report a new match")
	}
	// Re-recording the same swipe (e.g. a client retry) must not report a
	// second "new" match.
	second := s.recordSwipe(2, 1, true)
	if second {
		t.Fatalf("re-recording an already-matched swipe should not report a new match")
	}
}
