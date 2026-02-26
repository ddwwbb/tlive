package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/termlive/termlive/internal/daemon"
)

type sessionInfo struct {
	ID         string `json:"id"`
	Command    string `json:"command"`
	Pid        int    `json:"pid"`
	Status     string `json:"status"`
	Duration   string `json:"duration"`
	LastOutput string `json:"last_output"`
}

func handleSessionList(mgr *daemon.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessions := mgr.Store().List()
		infos := make([]sessionInfo, len(sessions))
		for i, s := range sessions {
			infos[i] = sessionInfo{
				ID:         s.ID,
				Command:    s.Command,
				Pid:        s.Pid,
				Status:     string(s.Status),
				Duration:   s.Duration().Truncate(time.Second).String(),
				LastOutput: string(s.LastOutput(200)),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(infos)
	}
}

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
