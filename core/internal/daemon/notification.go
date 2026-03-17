package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// NotificationType represents the category of a notification.
type NotificationType string

const (
	NotifyDone     NotificationType = "done"
	NotifyConfirm  NotificationType = "confirm"
	NotifyError    NotificationType = "error"
	NotifyProgress NotificationType = "progress"
)

// Notification represents a single notification event received by the daemon.
type Notification struct {
	ID        string           `json:"id"`
	Type      NotificationType `json:"type"`
	Message   string           `json:"message"`
	Context   string           `json:"context,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
}

// NotificationStore is a thread-safe, capped-size store for notifications.
// It keeps the most recent notifications up to the configured limit.
type NotificationStore struct {
	mu    sync.RWMutex
	items []Notification
	limit int
}

// NewNotificationStore creates a store that retains at most limit notifications.
func NewNotificationStore(limit int) *NotificationStore {
	return &NotificationStore{
		items: make([]Notification, 0, limit),
		limit: limit,
	}
}

// Add creates a new notification and appends it to the store.
// If the store exceeds its limit, the oldest notification is evicted.
func (s *NotificationStore) Add(typ NotificationType, message, context string) Notification {
	n := Notification{
		ID:        generateNotificationID(),
		Type:      typ,
		Message:   message,
		Context:   context,
		Timestamp: time.Now(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, n)
	if len(s.items) > s.limit {
		s.items = s.items[len(s.items)-s.limit:]
	}
	return n
}

// List returns up to limit notifications, most recent first.
func (s *NotificationStore) List(limit int) []Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.items)
	if limit < n {
		n = limit
	}
	result := make([]Notification, n)
	for i := 0; i < n; i++ {
		result[i] = s.items[len(s.items)-1-i]
	}
	return result
}

func generateNotificationID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "notif_" + hex.EncodeToString(b)
}
