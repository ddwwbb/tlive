package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/termlive/termlive/internal/hub"
	"github.com/termlive/termlive/internal/session"
)

func TestSessionListAPI(t *testing.T) {
	store := session.NewStore()
	s := session.New("echo", []string{"hello"})
	store.Add(s)
	h := hub.New()
	srv := New(store, map[string]*hub.Hub{s.ID: h}, "")
	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, s.ID) {
		t.Errorf("expected session ID in response, got: %s", body)
	}
}

func TestWebSocketConnection(t *testing.T) {
	store := session.NewStore()
	s := session.New("test", nil)
	store.Add(s)
	h := hub.New()
	go h.Run()
	defer h.Stop()
	srv := New(store, map[string]*hub.Hub{s.ID: h}, "")
	server := httptest.NewServer(srv.Handler())
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/" + s.ID
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	h.Broadcast([]byte("hello from pty"))
	ws.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != "hello from pty" {
		t.Errorf("expected 'hello from pty', got %q", string(msg))
	}
}
