package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/termlive/termlive/internal/daemon"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsControlMessage represents a JSON control message sent over WebSocket.
type wsControlMessage struct {
	Type string `json:"type"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

func handleWebSocket(mgr *daemon.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws/"), "/")
		sessionID := parts[0]
		h := mgr.Hub(sessionID)
		if h == nil {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewWSClient(conn)
		h.Register(client)
		defer func() {
			h.Unregister(client)
			client.Close()
		}()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var ctrl wsControlMessage
			if json.Unmarshal(msg, &ctrl) == nil && ctrl.Type == "resize" {
				if fn := mgr.ResizeFunc(sessionID); fn != nil {
					fn(ctrl.Rows, ctrl.Cols)
				}
				continue
			}
			h.Input(msg)
		}
	}
}
