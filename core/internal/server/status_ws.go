package server

import (
	"encoding/json"
	"net/http"
	"time"
)

const (
	statusVersion       = "0.1.0"
	statusSendInterval  = 5 * time.Second
)

// AggregatedStatus is the JSON payload sent over /ws/status WebSocket connections.
type AggregatedStatus struct {
	ActiveSessions int    `json:"active_sessions"`
	Version        string `json:"version"`
}

// buildStatus assembles the current aggregated status from the server's session manager.
func (s *Server) buildStatus() AggregatedStatus {
	return AggregatedStatus{
		ActiveSessions: s.mgr.ActiveCount(),
		Version:        statusVersion,
	}
}

// handleStatusWebSocket upgrades the connection to WebSocket and streams aggregated
// status JSON to the client — immediately on connect and then every 5 seconds.
func (s *Server) handleStatusWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// sendStatus marshals the current status and writes it as a text frame.
	sendStatus := func() error {
		payload, err := json.Marshal(s.buildStatus())
		if err != nil {
			return err
		}
		return conn.WriteMessage(1 /* TextMessage */, payload)
	}

	// Send initial status immediately.
	if err := sendStatus(); err != nil {
		return
	}

	ticker := time.NewTicker(statusSendInterval)
	defer ticker.Stop()

	// pumpRead drains client messages and detects disconnections.
	// Returns on first read error (normal closure, abnormal, etc.).
	readErr := make(chan error, 1)
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				readErr <- err
				return
			}
		}
	}()

	for {
		select {
		case <-ticker.C:
			if err := sendStatus(); err != nil {
				return
			}
		case <-readErr:
			// Client disconnected or sent an error frame.
			return
		}
	}
}
