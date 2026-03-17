// internal/session/session_test.go
package session

import (
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	s := New("claude", []string{})
	if s.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if s.Command != "claude" {
		t.Errorf("expected command 'claude', got %s", s.Command)
	}
	if s.Status != StatusRunning {
		t.Errorf("expected status Running, got %s", s.Status)
	}
	if s.StartTime.IsZero() {
		t.Error("expected non-zero start time")
	}
}

func TestStoreAddAndList(t *testing.T) {
	store := NewStore()
	s1 := New("cmd1", nil)
	s2 := New("cmd2", nil)
	store.Add(s1)
	store.Add(s2)
	sessions := store.List()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestStoreGet(t *testing.T) {
	store := NewStore()
	s := New("test", nil)
	store.Add(s)
	got, ok := store.Get(s.ID)
	if !ok {
		t.Error("expected to find session")
	}
	if got.ID != s.ID {
		t.Error("wrong session returned")
	}
	_, ok = store.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent session")
	}
}

func TestStoreRemove(t *testing.T) {
	store := NewStore()
	s := New("test", nil)
	store.Add(s)
	store.Remove(s.ID)
	if len(store.List()) != 0 {
		t.Error("expected empty store after remove")
	}
}

func TestSessionOutputBuffer(t *testing.T) {
	s := New("test", nil)
	s.AppendOutput([]byte("hello "))
	s.AppendOutput([]byte("world"))
	last := s.LastOutput(1024)
	if string(last) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(last))
	}
}

func TestSessionDuration(t *testing.T) {
	s := New("test", nil)
	s.StartTime = time.Now().Add(-5 * time.Minute)
	d := s.Duration()
	if d < 4*time.Minute || d > 6*time.Minute {
		t.Errorf("expected ~5m duration, got %v", d)
	}
}
