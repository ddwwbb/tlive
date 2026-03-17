package daemon

import (
	"testing"
)

func TestNotificationStore_AddAndList(t *testing.T) {
	store := NewNotificationStore(100)

	n1 := store.Add("done", "Task completed", "")
	if n1.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if n1.Type != "done" {
		t.Fatalf("expected type 'done', got %q", n1.Type)
	}

	n2 := store.Add("error", "Build failed", "see logs")

	list := store.List(10)
	if len(list) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(list))
	}
	// Most recent first
	if list[0].ID != n2.ID {
		t.Fatal("expected most recent notification first")
	}
}

func TestNotificationStore_Limit(t *testing.T) {
	store := NewNotificationStore(3)
	store.Add("done", "msg1", "")
	store.Add("done", "msg2", "")
	store.Add("done", "msg3", "")
	store.Add("done", "msg4", "")

	list := store.List(100)
	if len(list) != 3 {
		t.Fatalf("expected 3 notifications (capped), got %d", len(list))
	}
	// Oldest should be msg2 (msg1 evicted)
	if list[2].Message != "msg2" {
		t.Fatalf("expected oldest to be 'msg2', got %q", list[2].Message)
	}
}

func TestNotificationStore_ListLimit(t *testing.T) {
	store := NewNotificationStore(100)
	store.Add("done", "msg1", "")
	store.Add("done", "msg2", "")
	store.Add("done", "msg3", "")

	list := store.List(2)
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
}
