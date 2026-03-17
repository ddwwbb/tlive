package generator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/termlive/termlive/core/internal/config"
	"github.com/termlive/termlive/core/internal/daemon"
)

func TestIntegration_InitThenNotify(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// 1. Generate files
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 0})
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	// 2. Verify config is loadable
	cfgPath := filepath.Join(dir, ".termlive.toml")
	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Notify.Channels) == 0 {
		t.Fatal("expected at least one channel")
	}

	// 3. Start daemon on random port (using httptest)
	d := daemon.NewDaemon(daemon.DaemonConfig{
		Port:         0,
		Token:        "integration-test-token",
		HistoryLimit: 10,
	})
	ts := httptest.NewServer(d.Handler())
	defer ts.Close()

	// 4. Send notification
	body := `{"type":"done","message":"Integration test passed"}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/notify", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer integration-test-token")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// 5. Verify notification in history
	req2, _ := http.NewRequest("GET", ts.URL+"/api/notifications?limit=10", nil)
	req2.Header.Set("Authorization", "Bearer integration-test-token")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var result daemon.NotificationsResponse
	json.NewDecoder(resp2.Body).Decode(&result)
	if result.Total != 1 {
		t.Fatalf("expected 1 notification, got %d", result.Total)
	}
	if result.Notifications[0].Message != "Integration test passed" {
		t.Fatalf("unexpected message: %q", result.Notifications[0].Message)
	}

	// 6. Verify status endpoint
	req3, _ := http.NewRequest("GET", ts.URL+"/api/status", nil)
	req3.Header.Set("Authorization", "Bearer integration-test-token")
	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var status daemon.StatusResponse
	json.NewDecoder(resp3.Body).Decode(&status)
	if status.Status != "running" {
		t.Fatalf("expected status 'running', got %q", status.Status)
	}

	// 7. Verify generated files exist
	expectedFiles := gen.GeneratedFiles()
	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected file %s to exist", f)
		}
	}

	fmt.Printf("Integration test: init -> daemon -> notify -> verify: PASS\n")
}
