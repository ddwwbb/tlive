package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/termlive/termlive/core/internal/daemon"
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

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/session/" + ms.Session.ID
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	ms.Hub.Broadcast([]byte("hello from pty"))
	// Drain messages until we find the expected one; the session may replay
	// buffered PTY output (e.g. "hello\r\n") before our broadcast arrives.
	deadline := time.Now().Add(2 * time.Second)
	for {
		ws.SetReadDeadline(deadline)
		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if string(msg) == "hello from pty" {
			return
		}
	}
}

func TestWebSocketRouting_Session(t *testing.T) {
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

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/session/" + ms.Session.ID
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("expected successful WebSocket upgrade for /ws/session/<id>, got error: %v (resp: %v)", err, resp)
	}
	defer ws.Close()

	// Verify we get output — broadcast something and read it back.
	// Drain any replayed PTY output before checking for our broadcast.
	ms.Hub.Broadcast([]byte("routing test"))
	deadline := time.Now().Add(2 * time.Second)
	for {
		ws.SetReadDeadline(deadline)
		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if string(msg) == "routing test" {
			return
		}
	}
}

func TestWebSocketRouting_Status(t *testing.T) {
	mgr := daemon.NewSessionManager()
	srv := New(mgr)
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	// /ws/status should upgrade to WebSocket and send an initial status JSON message.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/status"
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("expected successful WebSocket upgrade for /ws/status, got error: %v (resp: %v)", err, resp)
	}
	defer ws.Close()

	// Read the initial status message.
	ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("expected initial status message, got error: %v", err)
	}

	// Verify the message contains expected fields.
	var status map[string]interface{}
	if err := json.Unmarshal(msg, &status); err != nil {
		t.Fatalf("expected valid JSON status message, got error: %v (msg: %s)", err, msg)
	}
	if _, ok := status["active_sessions"]; !ok {
		t.Errorf("expected 'active_sessions' field in status message, got: %v", status)
	}
	if _, ok := status["version"]; !ok {
		t.Errorf("expected 'version' field in status message, got: %v", status)
	}
}

func TestWebSocketRouting_Unknown(t *testing.T) {
	mgr := daemon.NewSessionManager()
	srv := New(mgr)
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	// /ws/random should return 404 — no handler registered for it.
	resp, err := http.Get(server.URL + "/ws/random")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for /ws/random, got %d", resp.StatusCode)
	}
}
