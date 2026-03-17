package daemon

import (
	"sync"
	"time"
)

// heartbeatTimeout is the duration after which a bridge is considered disconnected
// if no heartbeat has been received.
const heartbeatTimeout = 30 * time.Second

// BridgeInfo holds the registration and liveness info for a connected bridge.
type BridgeInfo struct {
	Version        string    `json:"version"`
	CoreMinVersion string    `json:"core_min_version"`
	Channels       []string  `json:"channels"`
	ConnectedAt    time.Time `json:"connected_at"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
	Connected      bool      `json:"connected"`
}

// BridgeManager tracks the Node.js Bridge registration and heartbeat state.
type BridgeManager struct {
	mu   sync.RWMutex
	info *BridgeInfo
}

// NewBridgeManager creates a new BridgeManager with no registered bridge.
func NewBridgeManager() *BridgeManager {
	return &BridgeManager{}
}

// Register records bridge registration details and marks it as connected.
func (bm *BridgeManager) Register(version, coreMinVersion string, channels []string) error {
	now := time.Now()
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.info = &BridgeInfo{
		Version:        version,
		CoreMinVersion: coreMinVersion,
		Channels:       channels,
		ConnectedAt:    now,
		LastHeartbeat:  now,
		Connected:      true,
	}
	return nil
}

// Heartbeat updates the last heartbeat timestamp for the registered bridge.
// It is a no-op if no bridge is registered.
func (bm *BridgeManager) Heartbeat() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.info == nil {
		return
	}
	bm.info.LastHeartbeat = time.Now()
}

// Status returns a copy of the current BridgeInfo. Connected is set to true only
// if a bridge is registered and its last heartbeat was within heartbeatTimeout.
func (bm *BridgeManager) Status() BridgeInfo {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	if bm.info == nil {
		return BridgeInfo{Connected: false}
	}
	info := *bm.info // copy
	info.Connected = time.Since(info.LastHeartbeat) < heartbeatTimeout
	return info
}

// IsConnected returns true if a bridge is registered and its heartbeat is recent.
func (bm *BridgeManager) IsConnected() bool {
	return bm.Status().Connected
}
