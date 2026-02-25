package daemon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/termlive/termlive/internal/notify"
)

type mockNotifier struct {
	sent []*notify.NotifyMessage
}

func (m *mockNotifier) Send(msg *notify.NotifyMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}

func TestDaemon_RelaysToNotifiers(t *testing.T) {
	mock := &mockNotifier{}
	d := NewDaemon(DaemonConfig{Port: 0, Token: "t"})
	d.SetNotifiers(notify.NewMultiNotifier(mock))

	body := `{"type":"done","message":"All tests passed"}`
	req := httptest.NewRequest("POST", "/api/notify", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 notification relayed, got %d", len(mock.sent))
	}
	if mock.sent[0].LastOutput != "All tests passed" {
		t.Fatalf("expected message 'All tests passed', got %q", mock.sent[0].LastOutput)
	}
}

func TestDaemon_NoNotifierConfigured(t *testing.T) {
	d := NewDaemon(DaemonConfig{Port: 0, Token: "t"})
	// No SetNotifiers called -- should not panic

	body := `{"type":"done","message":"test"}`
	req := httptest.NewRequest("POST", "/api/notify", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
