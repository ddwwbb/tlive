package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/termlive/termlive/internal/hub"
	"github.com/termlive/termlive/internal/session"
)

type sessionInfo struct {
	ID         string `json:"id"`
	Command    string `json:"command"`
	Pid        int    `json:"pid"`
	Status     string `json:"status"`
	Duration   string `json:"duration"`
	LastOutput string `json:"last_output"`
}

func handleSessionList(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessions := store.List()
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

func handleWebSocket(hubs map[string]*hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws/"), "/")
		sessionID := parts[0]
		h, ok := hubs[sessionID]
		if !ok {
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
			h.Input(msg)
		}
	}
}
