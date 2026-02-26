package server

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/termlive/termlive/internal/daemon"
	"github.com/termlive/termlive/internal/session"
)

func TestSessionListAPI(t *testing.T) {
	mgr := daemon.NewSessionManager()
	s := session.New("echo", []string{"hello"})
	mgr.Store().Add(s)
	srv := New(mgr)
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

func testCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", []string{"/C", "echo hello"}
	}
	return "echo", []string{"hello"}
}

func TestWebSocketConnection(t *testing.T) {
	mgr := daemon.NewSessionManager()
	cmd, args := testCommand()
	ms, err := mgr.CreateSession(cmd, args, daemon.SessionConfig{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.StopSession(ms.Session.ID)

	srv := New(mgr)
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/" + ms.Session.ID
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	ms.Hub.Broadcast([]byte("hello from pty"))
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != "hello from pty" {
		t.Errorf("expected 'hello from pty', got %q", string(msg))
	}
}
