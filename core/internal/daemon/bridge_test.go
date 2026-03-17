package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- BridgeManager unit tests ---

func TestBridgeManager_Register(t *testing.T) {
	bm := NewBridgeManager()

	// Initially not connected
	if bm.IsConnected() {
		t.Fatal("expected bridge to be disconnected initially")
	}

	err := bm.Register("1.0.0", "1.0.0", []string{"telegram"})
	if err != nil {
		t.Fatalf("unexpected error on Register: %v", err)
	}

	if !bm.IsConnected() {
		t.Fatal("expected bridge to be connected after Register")
	}

	status := bm.Status()
	if status.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %q", status.Version)
	}
	if status.CoreMinVersion != "1.0.0" {
		t.Errorf("expected core_min_version 1.0.0, got %q", status.CoreMinVersion)
	}
	if len(status.Channels) != 1 || status.Channels[0] != "telegram" {
		t.Errorf("expected channels [telegram], got %v", status.Channels)
	}
	if !status.Connected {
		t.Fatal("expected status.Connected to be true after Register")
	}
}

func TestBridgeManager_Heartbeat(t *testing.T) {
	bm := NewBridgeManager()

	err := bm.Register("1.0.0", "1.0.0", []string{"telegram"})
	if err != nil {
		t.Fatalf("unexpected error on Register: %v", err)
	}

	before := bm.Status().LastHeartbeat
	time.Sleep(5 * time.Millisecond)
	bm.Heartbeat()
	after := bm.Status().LastHeartbeat

	if !after.After(before) {
		t.Errorf("expected LastHeartbeat to advance after Heartbeat(), before=%v after=%v", before, after)
	}
}

func TestBridgeManager_Status(t *testing.T) {
	bm := NewBridgeManager()

	// Before register: connected should be false
	s := bm.Status()
	if s.Connected {
		t.Fatal("expected Connected=false before any registration")
	}

	// After register: connected should be true
	bm.Register("2.0.0", "1.5.0", []string{"slack", "telegram"})
	s = bm.Status()
	if !s.Connected {
		t.Fatal("expected Connected=true after registration")
	}
	if s.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %q", s.Version)
	}
	if len(s.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(s.Channels))
	}
}

func TestBridgeManager_HeartbeatTimeout(t *testing.T) {
	bm := NewBridgeManager()
	bm.Register("1.0.0", "1.0.0", []string{"telegram"})

	// Manually backdate last heartbeat to simulate a 30s+ gap
	bm.mu.Lock()
	bm.info.LastHeartbeat = time.Now().Add(-31 * time.Second)
	bm.mu.Unlock()

	s := bm.Status()
	if s.Connected {
		t.Fatal("expected Connected=false after 30s heartbeat timeout")
	}

	if bm.IsConnected() {
		t.Fatal("expected IsConnected()=false after 30s heartbeat timeout")
	}
}

// --- HTTP endpoint tests ---

func TestBridgeAPI_Register(t *testing.T) {
	d := NewDaemon(DaemonConfig{Port: 0, Token: "test-token"})
	handler := d.Handler()

	body := `{"version":"1.0.0","core_min_version":"1.0.0","channels":["telegram"]}`
	req := httptest.NewRequest("POST", "/api/bridge/register", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify bridge is now connected
	if !d.bridge.IsConnected() {
		t.Fatal("expected bridge to be connected after API register call")
	}
}

func TestBridgeAPI_Heartbeat(t *testing.T) {
	d := NewDaemon(DaemonConfig{Port: 0, Token: "test-token"})
	handler := d.Handler()

	// Register first
	regBody := `{"version":"1.0.0","core_min_version":"1.0.0","channels":["telegram"]}`
	regReq := httptest.NewRequest("POST", "/api/bridge/register", strings.NewReader(regBody))
	regReq.Header.Set("Authorization", "Bearer test-token")
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	handler.ServeHTTP(regW, regReq)
	if regW.Code != http.StatusOK {
		t.Fatalf("register returned %d: %s", regW.Code, regW.Body.String())
	}

	beforeHB := d.bridge.Status().LastHeartbeat
	time.Sleep(5 * time.Millisecond)

	// Send heartbeat
	req := httptest.NewRequest("POST", "/api/bridge/heartbeat", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	afterHB := d.bridge.Status().LastHeartbeat
	if !afterHB.After(beforeHB) {
		t.Errorf("expected LastHeartbeat to advance, before=%v after=%v", beforeHB, afterHB)
	}
}

func TestBridgeAPI_Status(t *testing.T) {
	d := NewDaemon(DaemonConfig{Port: 0, Token: "test-token"})
	handler := d.Handler()

	// GET status before register
	req := httptest.NewRequest("GET", "/api/bridge/status", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var statusBefore BridgeInfo
	if err := json.NewDecoder(w.Body).Decode(&statusBefore); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if statusBefore.Connected {
		t.Fatal("expected Connected=false before registration")
	}

	// Register bridge
	regBody := `{"version":"1.2.3","core_min_version":"1.0.0","channels":["telegram","slack"]}`
	regReq := httptest.NewRequest("POST", "/api/bridge/register", strings.NewReader(regBody))
	regReq.Header.Set("Authorization", "Bearer test-token")
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	handler.ServeHTTP(regW, regReq)
	if regW.Code != http.StatusOK {
		t.Fatalf("register returned %d: %s", regW.Code, regW.Body.String())
	}

	// GET status after register
	req2 := httptest.NewRequest("GET", "/api/bridge/status", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var statusAfter BridgeInfo
	if err := json.NewDecoder(w2.Body).Decode(&statusAfter); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !statusAfter.Connected {
		t.Fatal("expected Connected=true after registration")
	}
	if statusAfter.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", statusAfter.Version)
	}
	if len(statusAfter.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d: %v", len(statusAfter.Channels), statusAfter.Channels)
	}
}
