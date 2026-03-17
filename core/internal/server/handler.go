package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/termlive/termlive/core/internal/daemon"
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
		ms, ok := mgr.GetSession(sessionID)
		if !ok {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		h := ms.Hub
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewWSClient(conn)

		// Replay buffered output so the browser sees prior ANSI
		// style/color sequences and existing terminal content.
		if buf := ms.Session.LastOutput(64 * 1024); len(buf) > 0 {
			client.Send(buf)
		}

		h.Register(client)
		defer func() {
			h.Unregister(client)
			client.Close()
		}()

		// Watch for process exit and notify this client via text frame.
		go func() {
			<-ms.Done()
			exitMsg, _ := json.Marshal(map[string]interface{}{
				"type": "exit",
				"code": ms.ExitCode(),
			})
			client.SendText(exitMsg)
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
