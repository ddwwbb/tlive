package server

import (
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/termlive/termlive/internal/daemon"
)

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
