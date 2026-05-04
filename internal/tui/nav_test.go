package tui

import "testing"

func TestStackPushPop(t *testing.T) {
	var s ScreenStack
	if s.Len() != 0 {
		t.Fatal("new stack should be empty")
	}
	s.Push(nil)
	s.Push(nil)
	if s.Len() != 2 {
		t.Fatalf("expected len 2, got %d", s.Len())
	}
	if _, ok := s.Pop(); !ok {
		t.Fatal("pop should succeed")
	}
	if s.Len() != 1 {
		t.Fatalf("expected len 1, got %d", s.Len())
	}
	s.Replace(nil)
	if s.Len() != 1 {
		t.Fatalf("replace should leave len 1, got %d", s.Len())
	}
	if _, ok := s.Pop(); !ok {
		t.Fatal("pop should succeed after replace")
	}
	if _, ok := s.Pop(); ok {
		t.Fatal("pop on empty should fail")
	}
}
