# Notification Hub Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform TermLive from PTY-only monitoring into a three-layer notification hub (hooks + skills + PTY fallback) with `tlive init` and `tlive notify` commands.

**Architecture:** Daemon becomes a lightweight HTTP notification server. `tlive init` generates Claude Code skills/rules/hooks. `tlive notify` is a thin CLI that POSTs to the daemon API. Existing PTY/session/hub modules are kept for full mode.

**Tech Stack:** Go 1.24, cobra CLI, net/http, go-toml/v2, embed (templates)

---

### Task 1: Notification Model and Store

The foundation for all notification features. A `Notification` struct and a thread-safe `NotificationStore` with capped history.

**Files:**
- Create: `internal/daemon/notification.go`
- Create: `internal/daemon/notification_test.go`

**Step 1: Write the failing test**

```go
// internal/daemon/notification_test.go
package daemon

import (
	"testing"
)

func TestNotificationStore_AddAndList(t *testing.T) {
	store := NewNotificationStore(100)

	n1 := store.Add("done", "Task completed", "")
	if n1.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if n1.Type != "done" {
		t.Fatalf("expected type 'done', got %q", n1.Type)
	}

	n2 := store.Add("error", "Build failed", "see logs")

	list := store.List(10)
	if len(list) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(list))
	}
	// Most recent first
	if list[0].ID != n2.ID {
		t.Fatal("expected most recent notification first")
	}
}

func TestNotificationStore_Limit(t *testing.T) {
	store := NewNotificationStore(3)
	store.Add("done", "msg1", "")
	store.Add("done", "msg2", "")
	store.Add("done", "msg3", "")
	store.Add("done", "msg4", "")

	list := store.List(100)
	if len(list) != 3 {
		t.Fatalf("expected 3 notifications (capped), got %d", len(list))
	}
	// Oldest should be msg2 (msg1 evicted)
	if list[2].Message != "msg2" {
		t.Fatalf("expected oldest to be 'msg2', got %q", list[2].Message)
	}
}

func TestNotificationStore_ListLimit(t *testing.T) {
	store := NewNotificationStore(100)
	store.Add("done", "msg1", "")
	store.Add("done", "msg2", "")
	store.Add("done", "msg3", "")

	list := store.List(2)
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/ -run TestNotificationStore -v`
Expected: FAIL — types not defined

**Step 3: Write minimal implementation**

```go
// internal/daemon/notification.go
package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// NotificationType represents the category of a notification.
type NotificationType string

const (
	NotifyDone     NotificationType = "done"
	NotifyConfirm  NotificationType = "confirm"
	NotifyError    NotificationType = "error"
	NotifyProgress NotificationType = "progress"
)

// Notification represents a single notification event received by the daemon.
type Notification struct {
	ID        string           `json:"id"`
	Type      NotificationType `json:"type"`
	Message   string           `json:"message"`
	Context   string           `json:"context,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
}

// NotificationStore is a thread-safe, capped-size store for notifications.
// It keeps the most recent notifications up to the configured limit.
type NotificationStore struct {
	mu    sync.RWMutex
	items []Notification
	limit int
}

// NewNotificationStore creates a store that retains at most limit notifications.
func NewNotificationStore(limit int) *NotificationStore {
	return &NotificationStore{
		items: make([]Notification, 0, limit),
		limit: limit,
	}
}

// Add creates a new notification and appends it to the store.
// If the store exceeds its limit, the oldest notification is evicted.
func (s *NotificationStore) Add(typ NotificationType, message, context string) Notification {
	n := Notification{
		ID:        generateNotificationID(),
		Type:      typ,
		Message:   message,
		Context:   context,
		Timestamp: time.Now(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, n)
	if len(s.items) > s.limit {
		s.items = s.items[len(s.items)-s.limit:]
	}
	return n
}

// List returns up to limit notifications, most recent first.
func (s *NotificationStore) List(limit int) []Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.items)
	if limit < n {
		n = limit
	}
	result := make([]Notification, n)
	for i := 0; i < n; i++ {
		result[i] = s.items[len(s.items)-1-i]
	}
	return result
}

func generateNotificationID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "notif_" + hex.EncodeToString(b)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/daemon/ -run TestNotificationStore -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/daemon/notification.go internal/daemon/notification_test.go
git commit -m "feat: add NotificationStore for capped notification history"
```

---

### Task 2: Extend Config for Notification Hub

Add daemon and notification hub fields to the config struct. Support `.termlive.toml`.

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile_WithDaemonConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".termlive.toml")
	content := `
[daemon]
port = 9090
token = "my-token"

[notify]
channels = ["web", "wechat"]
short_timeout = 15

[notify.options]
include_context = false
history_limit = 50

[notify.wechat]
webhook_url = "https://example.com/hook"
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Daemon.Port != 9090 {
		t.Fatalf("expected daemon port 9090, got %d", cfg.Daemon.Port)
	}
	if cfg.Daemon.Token != "my-token" {
		t.Fatalf("expected token 'my-token', got %q", cfg.Daemon.Token)
	}
	if len(cfg.Notify.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(cfg.Notify.Channels))
	}
	if cfg.Notify.Options.HistoryLimit != 50 {
		t.Fatalf("expected history_limit 50, got %d", cfg.Notify.Options.HistoryLimit)
	}
	if cfg.Notify.Options.IncludeContext != false {
		t.Fatal("expected include_context false")
	}
}

func TestDefault_HasSaneDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Daemon.Port != 8080 {
		t.Fatalf("expected default daemon port 8080, got %d", cfg.Daemon.Port)
	}
	if cfg.Notify.Options.HistoryLimit != 100 {
		t.Fatalf("expected default history_limit 100, got %d", cfg.Notify.Options.HistoryLimit)
	}
	if cfg.Notify.Options.IncludeContext != true {
		t.Fatal("expected default include_context true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadFromFile_WithDaemonConfig -v`
Expected: FAIL — `cfg.Daemon` field does not exist

**Step 3: Write minimal implementation**

Update `internal/config/config.go` — add `DaemonConfig`, `NotifyOptions`, `Channels` fields:

```go
// internal/config/config.go
package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Config is the top-level configuration for TermLive.
type Config struct {
	Daemon DaemonConfig `toml:"daemon"`
	Server ServerConfig `toml:"server"`
	Notify NotifyConfig `toml:"notify"`
}

// DaemonConfig holds settings for the background daemon process.
type DaemonConfig struct {
	Port      int    `toml:"port"`
	Token     string `toml:"token"`
	AutoStart bool   `toml:"auto_start"`
}

// ServerConfig holds settings for the HTTP/WebSocket server.
type ServerConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

// NotifyConfig holds settings for notification channels and idle detection.
type NotifyConfig struct {
	Channels     []string      `toml:"channels"`
	ShortTimeout int           `toml:"short_timeout"` // seconds, for awaiting-input (default 30)
	LongTimeout  int           `toml:"long_timeout"`  // seconds, for unknown idle (default 120)
	Options      NotifyOptions `toml:"options"`
	Patterns     PatternConfig `toml:"patterns"`
	WeChat       WeChatConfig  `toml:"wechat"`
	Feishu       FeishuConfig  `toml:"feishu"`
}

// NotifyOptions controls notification behavior.
type NotifyOptions struct {
	IncludeContext bool `toml:"include_context"`
	HistoryLimit   int  `toml:"history_limit"`
}

// PatternConfig holds custom patterns for output classification.
type PatternConfig struct {
	AwaitingInput []string `toml:"awaiting_input"`
	Processing    []string `toml:"processing"`
}

// WeChatConfig holds WeChat webhook settings.
type WeChatConfig struct {
	WebhookURL string `toml:"webhook_url"`
}

// FeishuConfig holds Feishu webhook settings.
type FeishuConfig struct {
	WebhookURL string `toml:"webhook_url"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Daemon: DaemonConfig{Port: 8080},
		Server: ServerConfig{Port: 8080, Host: "0.0.0.0"},
		Notify: NotifyConfig{
			Channels:     []string{"web"},
			ShortTimeout: 30,
			LongTimeout:  120,
			Options: NotifyOptions{
				IncludeContext: true,
				HistoryLimit:   100,
			},
		},
	}
}

// LoadFromFile reads a TOML config from path, falling back to defaults.
func LoadFromFile(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS

**Step 5: Verify run.go still compiles**

Run: `go build ./cmd/tlive/`
Expected: SUCCESS (run.go uses `cfg.Server.Port` and `cfg.Notify.*` which are still present)

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: extend config with daemon and notification options"
```

---

### Task 3: Rewrite Daemon as HTTP Server

Replace the JSON-RPC/socket daemon with a lightweight HTTP daemon. The daemon hosts the notification API and optionally the Web UI.

**Files:**
- Rewrite: `internal/daemon/daemon.go`
- Create: `internal/daemon/daemon_test.go` (rewrite)
- Remove: `internal/daemon/ipc.go`
- Remove: `internal/daemon/socket.go`
- Remove: `internal/daemon/socket_unix.go`
- Remove: `internal/daemon/socket_windows.go`

**Step 1: Write the failing test**

```go
// internal/daemon/daemon_test.go
package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDaemon_NotifyEndpoint(t *testing.T) {
	d := NewDaemon(DaemonConfig{Port: 0, Token: "test-token"})
	handler := d.Handler()

	// POST /api/notify without auth → 401
	req := httptest.NewRequest("POST", "/api/notify", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// POST /api/notify with auth → 200
	body := `{"type":"done","message":"Task completed"}`
	req = httptest.NewRequest("POST", "/api/notify", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp NotifyResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ID == "" {
		t.Fatal("expected non-empty notification ID")
	}
}

func TestDaemon_NotificationsEndpoint(t *testing.T) {
	d := NewDaemon(DaemonConfig{Port: 0, Token: "test-token"})
	d.notifications.Add("done", "msg1", "")
	d.notifications.Add("error", "msg2", "")

	handler := d.Handler()
	req := httptest.NewRequest("GET", "/api/notifications?limit=10", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp NotificationsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Total)
	}
	if len(resp.Notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(resp.Notifications))
	}
}

func TestDaemon_StatusEndpoint(t *testing.T) {
	d := NewDaemon(DaemonConfig{Port: 8080, Token: "t"})
	handler := d.Handler()

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp StatusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "running" {
		t.Fatalf("expected status 'running', got %q", resp.Status)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/ -run TestDaemon_ -v`
Expected: FAIL — compilation error, types not defined

**Step 3: Delete old IPC/socket files**

```bash
rm internal/daemon/ipc.go internal/daemon/socket.go internal/daemon/socket_unix.go internal/daemon/socket_windows.go
```

**Step 4: Write the new daemon**

```go
// internal/daemon/daemon.go
package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DaemonConfig holds configuration for the daemon HTTP server.
type DaemonConfig struct {
	Port         int
	Token        string
	HistoryLimit int
}

// Daemon is the TermLive notification hub. It receives notifications via
// HTTP API and relays them to configured channels (WeChat, Feishu, Web UI).
type Daemon struct {
	cfg           DaemonConfig
	mgr           *SessionManager
	notifications *NotificationStore
	token         string
	startTime     time.Time
	server        *http.Server
	mu            sync.Mutex
}

// NewDaemon creates a new Daemon with the given config.
func NewDaemon(cfg DaemonConfig) *Daemon {
	token := cfg.Token
	if token == "" {
		b := make([]byte, 16)
		rand.Read(b)
		token = hex.EncodeToString(b)
	}
	historyLimit := cfg.HistoryLimit
	if historyLimit <= 0 {
		historyLimit = 100
	}
	return &Daemon{
		cfg:           cfg,
		mgr:           NewSessionManager(),
		notifications: NewNotificationStore(historyLimit),
		token:         token,
		startTime:     time.Now(),
	}
}

// Manager returns the session manager (used by full mode for PTY sessions).
func (d *Daemon) Manager() *SessionManager { return d.mgr }

// Token returns the authentication token.
func (d *Daemon) Token() string { return d.token }

// Notifications returns the notification store (for direct internal use).
func (d *Daemon) Notifications() *NotificationStore { return d.notifications }

// --- HTTP API types ---

// NotifyRequest is the JSON body for POST /api/notify.
type NotifyRequest struct {
	Type    NotificationType `json:"type"`
	Message string           `json:"message"`
	Context string           `json:"context,omitempty"`
}

// NotifyResponse is the JSON response for POST /api/notify.
type NotifyResponse struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
}

// NotificationsResponse is the JSON response for GET /api/notifications.
type NotificationsResponse struct {
	Notifications []Notification `json:"notifications"`
	Total         int            `json:"total"`
}

// StatusResponse is the JSON response for GET /api/status.
type StatusResponse struct {
	Status   string `json:"status"`
	Uptime   string `json:"uptime"`
	Port     int    `json:"port"`
	Sessions int    `json:"sessions"`
}

// Handler returns the HTTP handler for the daemon API.
// This is separated from Run so it can be tested with httptest.
func (d *Daemon) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/notify", d.handleNotify)
	mux.HandleFunc("/api/notifications", d.handleNotifications)
	mux.HandleFunc("/api/status", d.handleStatus)
	return d.authMiddleware(mux)
}

// Run starts the HTTP server and blocks until Stop is called.
func (d *Daemon) Run() error {
	addr := fmt.Sprintf(":%d", d.cfg.Port)
	d.mu.Lock()
	d.server = &http.Server{Addr: addr, Handler: d.Handler()}
	d.mu.Unlock()
	log.Printf("TermLive daemon listening on %s", addr)
	if err := d.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop gracefully shuts down the daemon HTTP server.
func (d *Daemon) Stop() error {
	d.mu.Lock()
	srv := d.server
	d.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// --- Handlers ---

func (d *Daemon) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req NotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Type == "" || req.Message == "" {
		http.Error(w, "type and message are required", http.StatusBadRequest)
		return
	}
	n := d.notifications.Add(req.Type, req.Message, req.Context)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(NotifyResponse{
		ID:        n.ID,
		Timestamp: n.Timestamp,
	})
}

func (d *Daemon) handleNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items := d.notifications.List(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(NotificationsResponse{
		Notifications: items,
		Total:         len(items),
	})
}

func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessions := d.mgr.ListSessions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{
		Status:   "running",
		Uptime:   time.Since(d.startTime).Truncate(time.Second).String(),
		Port:     d.cfg.Port,
		Sessions: len(sessions),
	})
}

// --- Auth middleware ---

func (d *Daemon) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token != d.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/daemon/ -run TestDaemon_ -v`
Expected: PASS

**Step 6: Commit**

```bash
git rm internal/daemon/ipc.go internal/daemon/socket.go internal/daemon/socket_unix.go internal/daemon/socket_windows.go
git add internal/daemon/daemon.go internal/daemon/daemon_test.go
git commit -m "feat: rewrite daemon as HTTP notification hub

Replace JSON-RPC/socket IPC with HTTP API. Add /api/notify,
/api/notifications, and /api/status endpoints with Bearer auth."
```

---

### Task 4: Generator Framework and Claude Code Adapter

Creates `tlive init` file generation logic. The `Generator` interface allows future adapters (Cursor, Aider). The Claude Code adapter generates SKILL.md, CLAUDE.md rules, hooks config, and .termlive.toml.

**Files:**
- Create: `internal/generator/generator.go`
- Create: `internal/generator/claude_code.go`
- Create: `internal/generator/claude_code_test.go`

**Step 1: Write the failing test**

```go
// internal/generator/claude_code_test.go
package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeCodeGenerator_Generate(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{
		DaemonPort: 9090,
	})

	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	// Check SKILL.md was created
	skillPath := filepath.Join(dir, ".claude", "skills", "termlive-notify", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("SKILL.md not created: %v", err)
	}
	if !strings.Contains(string(data), "name: termlive-notify") {
		t.Fatal("SKILL.md missing frontmatter")
	}
	if !strings.Contains(string(data), "tlive notify") {
		t.Fatal("SKILL.md missing tlive notify command")
	}

	// Check settings.local.json was created
	settingsPath := filepath.Join(dir, ".claude", "settings.local.json")
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.local.json not created: %v", err)
	}
	if !strings.Contains(string(data), "PostToolUse") {
		t.Fatal("settings.local.json missing hooks")
	}

	// Check .termlive.toml was created
	configPath := filepath.Join(dir, ".termlive.toml")
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf(".termlive.toml not created: %v", err)
	}
	if !strings.Contains(string(data), "port = 9090") {
		t.Fatal(".termlive.toml missing daemon port")
	}
}

func TestClaudeCodeGenerator_AppendsClaudeMD(t *testing.T) {
	dir := t.TempDir()

	// Pre-existing CLAUDE.md
	existing := "# My Project\n\nSome existing rules.\n"
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(existing), 0644)

	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 8080})
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "# My Project") {
		t.Fatal("existing CLAUDE.md content was overwritten")
	}
	if !strings.Contains(content, "TermLive Notification Rules") {
		t.Fatal("CLAUDE.md missing TermLive rules")
	}
}

func TestClaudeCodeGenerator_Idempotent(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 8080})

	// Run twice
	gen.Generate()
	gen.Generate()

	// CLAUDE.md should not have duplicate rules
	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	count := strings.Count(string(data), "TermLive Notification Rules")
	if count != 1 {
		t.Fatalf("expected 1 occurrence of TermLive rules, got %d", count)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/generator/ -v`
Expected: FAIL — package does not exist

**Step 3: Write the generator interface**

```go
// internal/generator/generator.go
package generator

// GeneratorConfig holds common settings used by all generators.
type GeneratorConfig struct {
	DaemonPort int    // Port the daemon will listen on
	Token      string // Auth token (auto-generated if empty)
}

// Generator is the interface for AI tool-specific file generators.
// Each AI tool (Claude Code, Cursor, etc.) implements this interface
// to produce its own rules/skills/hooks files.
type Generator interface {
	// Name returns the human-readable name of the AI tool.
	Name() string

	// Generate creates all necessary files in the project directory.
	// It should be idempotent — safe to run multiple times.
	Generate() error
}
```

**Step 4: Write the Claude Code adapter**

```go
// internal/generator/claude_code.go
package generator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeCodeGenerator generates skills, rules, hooks, and config
// files for Claude Code integration.
type ClaudeCodeGenerator struct {
	projectDir string
	cfg        GeneratorConfig
}

// NewClaudeCodeGenerator creates a generator targeting the given project directory.
func NewClaudeCodeGenerator(projectDir string, cfg GeneratorConfig) *ClaudeCodeGenerator {
	if cfg.Token == "" {
		b := make([]byte, 16)
		rand.Read(b)
		cfg.Token = hex.EncodeToString(b)
	}
	return &ClaudeCodeGenerator{projectDir: projectDir, cfg: cfg}
}

func (g *ClaudeCodeGenerator) Name() string { return "Claude Code" }

// Generate creates all Claude Code integration files.
func (g *ClaudeCodeGenerator) Generate() error {
	steps := []func() error{
		g.generateSkill,
		g.generateHooks,
		g.generateRules,
		g.generateConfig,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// GeneratedFiles returns the list of files that were (or will be) created.
func (g *ClaudeCodeGenerator) GeneratedFiles() []string {
	return []string{
		filepath.Join(".claude", "skills", "termlive-notify", "SKILL.md"),
		filepath.Join(".claude", "settings.local.json"),
		"CLAUDE.md",
		".termlive.toml",
	}
}

func (g *ClaudeCodeGenerator) generateSkill() error {
	dir := filepath.Join(g.projectDir, ".claude", "skills", "termlive-notify")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}
	content := `---
name: termlive-notify
description: Use when a task completes, needs user confirmation, or you want to report progress to the user
---

# TermLive Notify

## When to Use
- Task completed or milestone reached
- Need user confirmation or decision
- Encountered an error that requires user attention
- Long-running task progress update

## How to Notify
Run via Bash tool:

` + "```bash" + `
tlive notify --type done --message "Completed: <summary>"
tlive notify --type confirm --message "Need approval: <details>"
tlive notify --type error --message "Error: <details>"
tlive notify --type progress --message "Progress: <details>"
` + "```" + `
`
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)
}

func (g *ClaudeCodeGenerator) generateHooks() error {
	dir := filepath.Join(g.projectDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	content := `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "AskUserQuestion",
        "hooks": [
          {
            "type": "command",
            "command": "tlive notify --type confirm --message \"AI is waiting for your input\"",
            "timeout": 5000
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "tlive notify --type done --message \"Session ended\"",
            "timeout": 5000
          }
        ]
      }
    ]
  }
}
`
	return os.WriteFile(filepath.Join(dir, "settings.local.json"), []byte(content), 0644)
}

const termliveMark = "## TermLive Notification Rules"

func (g *ClaudeCodeGenerator) generateRules() error {
	path := filepath.Join(g.projectDir, "CLAUDE.md")
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	// Idempotent: skip if already present
	if strings.Contains(existing, termliveMark) {
		return nil
	}

	rules := `
` + termliveMark + `
- When you complete a significant task, invoke the termlive-notify skill
- When you need user confirmation, invoke the termlive-notify skill
- When you encounter a blocking error, invoke the termlive-notify skill
`
	content := existing + rules
	return os.WriteFile(path, []byte(content), 0644)
}

func (g *ClaudeCodeGenerator) generateConfig() error {
	path := filepath.Join(g.projectDir, ".termlive.toml")
	content := fmt.Sprintf(`# TermLive configuration
# See: https://github.com/termlive/termlive

[daemon]
port = %d
auto_start = false

[notify]
channels = ["web"]
# wechat_webhook = ""
# feishu_webhook = ""

[notify.options]
include_context = true
history_limit = 100
`, g.cfg.DaemonPort)
	return os.WriteFile(path, []byte(content), 0644)
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/generator/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/generator/generator.go internal/generator/claude_code.go internal/generator/claude_code_test.go
git commit -m "feat: add generator framework with Claude Code adapter

Generates SKILL.md, settings.local.json hooks, CLAUDE.md rules,
and .termlive.toml config. Idempotent — safe to run multiple times."
```

---

### Task 5: `tlive init` CLI Command

Wire the generator into a cobra command.

**Files:**
- Create: `cmd/tlive/init.go`
- Modify: `cmd/tlive/main.go` (add subcommand)

**Step 1: Write the init command**

```go
// cmd/tlive/init.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/termlive/termlive/internal/generator"
)

var (
	initTool string
	initYes  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize TermLive in the current project",
	Long: `Generate skills, rules, hooks, and config files for AI tool integration.

Currently supports: claude-code (default).
Run with --yes to skip interactive prompts.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initTool, "tool", "claude-code", "AI tool to configure (claude-code)")
	initCmd.Flags().BoolVar(&initYes, "yes", false, "Skip interactive prompts, use defaults")
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := generator.GeneratorConfig{
		DaemonPort: port,
	}

	var gen generator.Generator
	switch initTool {
	case "claude-code":
		gen = generator.NewClaudeCodeGenerator(dir, cfg)
	default:
		return fmt.Errorf("unsupported tool: %s (supported: claude-code)", initTool)
	}

	if !initYes {
		fmt.Fprintf(os.Stderr, "  Initializing TermLive for %s...\n\n", gen.Name())
	}

	if err := gen.Generate(); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	// Print generated files
	fmt.Fprintf(os.Stderr, "  Generated files:\n")
	for _, f := range gen.(*generator.ClaudeCodeGenerator).GeneratedFiles() {
		fmt.Fprintf(os.Stderr, "    %s\n", f)
	}
	fmt.Fprintf(os.Stderr, "\n  Run 'tlive daemon start' to start the notification service.\n")

	return nil
}
```

**Step 2: Update main.go to use subcommands**

Refactor `cmd/tlive/main.go` to support both `tlive init` and `tlive run <cmd>`:

```go
// cmd/tlive/main.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var port int

var rootCmd = &cobra.Command{
	Use:   "tlive",
	Short: "TermLive - Terminal live monitoring with AI notifications",
	Long: `TermLive wraps terminal commands for remote monitoring, interaction,
and intelligent notifications via AI tool integration (skills/hooks).`,
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 8080, "Web server / daemon port")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(notifyCmd)
	rootCmd.AddCommand(daemonCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 3: Rename runCommand to runCmd and wrap as subcommand**

Update `cmd/tlive/run.go`: change the top-level command into a `run` subcommand. Replace the `rootCmd` variable reference — the `runCommand` function stays the same, just attached to a new cobra.Command:

```go
// At the top of cmd/tlive/run.go, replace the old variable declarations with:
var (
	shortTimeout int
	longTimeout  int
	publicIP     string
)

var runCmd = &cobra.Command{
	Use:   "run <command> [args...]",
	Short: "Run a command with PTY wrapping and Web UI (full mode)",
	Long:  "Start a command inside a PTY with remote Web UI, notifications, and idle detection.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runCommand,
}

func init() {
	runCmd.Flags().IntVarP(&shortTimeout, "short-timeout", "s", 30, "Short idle timeout for detected prompts (seconds)")
	runCmd.Flags().IntVarP(&longTimeout, "long-timeout", "l", 120, "Long idle timeout for unknown idle (seconds)")
	runCmd.Flags().StringVar(&publicIP, "ip", "", "Override auto-detected LAN IP address")
}
```

The `runCommand` function body stays identical. Just update `cfg.Server.Port = port` since `port` is now a persistent flag on rootCmd.

**Step 4: Verify build succeeds**

Run: `go build ./cmd/tlive/`
Expected: SUCCESS (note: `notifyCmd` and `daemonCmd` not yet defined — create stubs first)

Create temporary stubs so the build succeeds:

```go
// cmd/tlive/notify.go
package main

import "github.com/spf13/cobra"

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Send a notification to the TermLive daemon",
	RunE:  func(cmd *cobra.Command, args []string) error { return nil },
}
```

```go
// cmd/tlive/daemon.go
package main

import "github.com/spf13/cobra"

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the TermLive daemon",
}
```

**Step 5: Test the init command**

Run: `go build ./cmd/tlive/ && ./tlive init --yes`
Expected: Files generated in current directory

**Step 6: Commit**

```bash
git add cmd/tlive/main.go cmd/tlive/run.go cmd/tlive/init.go cmd/tlive/notify.go cmd/tlive/daemon.go
git commit -m "feat: add tlive init command and restructure CLI as subcommands

tlive init generates Claude Code skills/rules/hooks/config.
CLI restructured: tlive run, tlive init, tlive notify, tlive daemon."
```

---

### Task 6: `tlive notify` CLI Command

The thin CLI wrapper that sends notifications to the daemon via HTTP POST.

**Files:**
- Rewrite: `cmd/tlive/notify.go`

**Step 1: Write the notify command**

```go
// cmd/tlive/notify.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/termlive/termlive/internal/config"
)

var (
	notifyType    string
	notifyMessage string
	notifyContext string
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Send a notification to the TermLive daemon",
	Long: `Send a notification to the running TermLive daemon.
Used by AI tool skills/hooks to trigger user notifications.

Exits silently if the daemon is not running (never blocks AI tools).`,
	RunE: runNotify,
}

func init() {
	notifyCmd.Flags().StringVar(&notifyType, "type", "progress", "Notification type: done, confirm, error, progress")
	notifyCmd.Flags().StringVarP(&notifyMessage, "message", "m", "", "Notification message (required)")
	notifyCmd.Flags().StringVar(&notifyContext, "context", "", "Additional context")
}

func runNotify(cmd *cobra.Command, args []string) error {
	if notifyMessage == "" {
		return fmt.Errorf("--message is required")
	}

	// Load config to find daemon address and token
	cfg := loadNotifyConfig()

	payload := map[string]string{
		"type":    notifyType,
		"message": notifyMessage,
	}
	if notifyContext != "" {
		payload["context"] = notifyContext
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("http://localhost:%d/api/notify", cfg.Daemon.Port)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Daemon.Token)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Daemon not running — fail silently, only hint on stderr
		fmt.Fprintln(os.Stderr, "tlive: daemon not running (notification not sent)")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Fprintln(os.Stderr, "tlive: authentication failed — try running 'tlive init' again")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "tlive: daemon returned status %d\n", resp.StatusCode)
	}
	return nil
}

// loadNotifyConfig loads .termlive.toml from the current directory,
// falling back to defaults if not found.
func loadNotifyConfig() *config.Config {
	cfg, err := config.LoadFromFile(".termlive.toml")
	if err != nil {
		return config.Default()
	}
	return cfg
}
```

**Step 2: Build and manual test**

Run: `go build ./cmd/tlive/`
Expected: SUCCESS

Run: `./tlive notify --type done --message "test notification"`
Expected: stderr "daemon not running" (since no daemon is started)

**Step 3: Commit**

```bash
git add cmd/tlive/notify.go
git commit -m "feat: add tlive notify CLI command

Thin HTTP client that POSTs to the daemon API.
Fails silently when daemon is not running to avoid blocking AI tools."
```

---

### Task 7: `tlive daemon start/stop` CLI Commands

Wire the daemon lifecycle into cobra subcommands.

**Files:**
- Rewrite: `cmd/tlive/daemon.go`

**Step 1: Write the daemon commands**

```go
// cmd/tlive/daemon.go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/termlive/termlive/internal/config"
	"github.com/termlive/termlive/internal/daemon"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the TermLive background daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the TermLive daemon (notification hub + Web UI)",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running TermLive daemon",
	RunE:  runDaemonStop,
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	cfg, _ := config.LoadFromFile(".termlive.toml")

	// CLI flag overrides config
	daemonPort := cfg.Daemon.Port
	if cmd.Flags().Changed("port") {
		daemonPort = port
	}

	d := daemon.NewDaemon(daemon.DaemonConfig{
		Port:         daemonPort,
		Token:        cfg.Daemon.Token,
		HistoryLimit: cfg.Notify.Options.HistoryLimit,
	})

	fmt.Fprintf(os.Stderr, "  TermLive daemon starting...\n")
	fmt.Fprintf(os.Stderr, "    Port:  %d\n", daemonPort)
	fmt.Fprintf(os.Stderr, "    Token: %s\n", d.Token())
	fmt.Fprintf(os.Stderr, "    API:   http://localhost:%d/api/\n\n", daemonPort)

	// Graceful shutdown on signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\n  Shutting down daemon...\n")
		d.Stop()
	}()

	return d.Run()
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	// For now, daemon stop sends a signal. A PID file approach can be added later.
	fmt.Fprintln(os.Stderr, "tlive: use Ctrl+C or kill the daemon process to stop it")
	fmt.Fprintln(os.Stderr, "  (PID file based stop will be added in a future version)")
	return nil
}
```

**Step 2: Build and test**

Run: `go build ./cmd/tlive/`
Expected: SUCCESS

Run: `./tlive daemon start &` then `./tlive notify --type done --message "it works"`
Expected: Notification received by daemon (no stderr warning)

**Step 3: Commit**

```bash
git add cmd/tlive/daemon.go
git commit -m "feat: add tlive daemon start/stop commands

Daemon runs as foreground HTTP server with graceful signal handling."
```

---

### Task 8: Wire Notification Relay to Notifiers

When the daemon receives a notification via API, relay it to configured channels (WeChat, Feishu).

**Files:**
- Modify: `internal/daemon/daemon.go` (add notifier relay)
- Create: `internal/daemon/relay_test.go`

**Step 1: Write the failing test**

```go
// internal/daemon/relay_test.go
package daemon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/termlive/termlive/internal/notify"
)

// mockNotifier records calls for testing.
type mockNotifier struct {
	sent []*notify.NotifyMessage
}

func (m *mockNotifier) Send(msg *notify.NotifyMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}

func TestDaemon_RelaysToNotifiers(t *testing.T) {
	mock := &mockNotifier{}
	d := NewDaemon(DaemonConfig{Port: 0, Token: "t"})
	d.SetNotifiers(notify.NewMultiNotifier(mock))

	body := `{"type":"done","message":"All tests passed"}`
	req := httptest.NewRequest("POST", "/api/notify", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 notification relayed, got %d", len(mock.sent))
	}
	if mock.sent[0].Command != "done" {
		t.Fatalf("expected command 'done', got %q", mock.sent[0].Command)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/ -run TestDaemon_Relays -v`
Expected: FAIL — `SetNotifiers` method not found

**Step 3: Add notifier relay to daemon**

Add to `internal/daemon/daemon.go`:

1. Add field `notifier *notify.MultiNotifier` to `Daemon` struct
2. Add method:
```go
// SetNotifiers configures external notification channels (WeChat, Feishu, etc.).
func (d *Daemon) SetNotifiers(n *notify.MultiNotifier) {
	d.notifier = n
}
```
3. In `handleNotify`, after `d.notifications.Add(...)`, add:
```go
// Relay to external notification channels
if d.notifier != nil {
	relayMsg := &notify.NotifyMessage{
		Command:    string(req.Type),
		LastOutput: req.Message,
		Confidence: "high",
	}
	if req.Context != "" {
		relayMsg.LastOutput = req.Message + "\n\n" + req.Context
	}
	go d.notifier.Send(relayMsg)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/daemon/ -run TestDaemon_Relays -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/daemon/daemon.go internal/daemon/relay_test.go
git commit -m "feat: relay notifications to WeChat/Feishu channels

Daemon forwards incoming API notifications to configured notifiers
asynchronously to avoid blocking the API response."
```

---

### Task 9: Wire Daemon into Full Mode (run.go)

Update `run.go` so that `tlive run` uses the new HTTP daemon instead of the old direct HTTP server. In full mode, the daemon hosts both the notification API and the existing Web UI + WebSocket terminal.

**Files:**
- Modify: `cmd/tlive/run.go`

**Step 1: Update run.go**

Key changes:
1. Create a `Daemon` instead of a standalone `server.Server`
2. Mount existing Web UI routes onto the daemon's mux
3. The daemon serves both `/api/notify` and `/ws/` + static files
4. `localOutputClient` and idle detection remain the same

The daemon's `Handler()` method needs to be extended to accept additional route registration. Add a method to `Daemon`:

```go
// MountHandler adds an additional handler to the daemon's HTTP mux.
// Must be called before Run().
func (d *Daemon) MountHandler(pattern string, handler http.Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.extraRoutes = append(d.extraRoutes, route{pattern, handler})
}
```

Then in `run.go`, replace the standalone `server.New(...)` + `httpServer` with:
```go
d := daemon.NewDaemon(daemon.DaemonConfig{
	Port:         cfg.Server.Port,
	HistoryLimit: cfg.Notify.Options.HistoryLimit,
})
// Mount existing Web UI and WebSocket on the daemon
srv := server.New(d.Manager().Store(), d.Manager().Hubs(), d.Token())
// ... rest of setup using d.Run() instead of httpServer.ListenAndServe()
```

**Step 2: Build and test**

Run: `go build ./cmd/tlive/`
Expected: SUCCESS

Run: `./tlive run echo hello`
Expected: Works as before — PTY output, Web UI accessible

**Step 3: Commit**

```bash
git add cmd/tlive/run.go internal/daemon/daemon.go
git commit -m "feat: wire full mode to use new HTTP daemon

tlive run now starts daemon internally, providing both Web UI
and notification API on the same port."
```

---

### Task 10: Cleanup and Integration Test

Remove dead code, add an integration test for the init → notify flow.

**Files:**
- Create: `internal/generator/integration_test.go`
- Modify: `go.mod` (tidy)

**Step 1: Write integration test**

```go
// internal/generator/integration_test.go
package generator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/termlive/termlive/internal/config"
	"github.com/termlive/termlive/internal/daemon"
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

	// 3. Start daemon on random port
	d := daemon.NewDaemon(daemon.DaemonConfig{
		Port:         0,
		Token:        "integration-test-token",
		HistoryLimit: 10,
	})

	// Use httptest to get a random port
	handler := d.Handler()
	ts := httptest.NewServer(handler)
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
}
```

Add missing import: `"net/http/httptest"`

**Step 2: Run integration test**

Run: `go test ./internal/generator/ -run TestIntegration -v`
Expected: PASS

**Step 3: Tidy dependencies**

Run: `go mod tidy`

**Step 4: Run all tests**

Run: `go test ./...`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/generator/integration_test.go go.mod go.sum
git commit -m "test: add integration test for init-then-notify flow

Verifies end-to-end: generator creates config, daemon accepts
notifications, and history API returns them correctly."
```

---

## Summary

| Task | What | Files | Estimated Steps |
|------|------|-------|-----------------|
| 1 | NotificationStore | 2 new | 5 |
| 2 | Config extension | 1 modify + 1 new | 6 |
| 3 | Daemon rewrite (HTTP) | 1 rewrite + 4 remove | 6 |
| 4 | Generator + Claude Code adapter | 3 new | 6 |
| 5 | `tlive init` command | 2 new + 1 modify | 6 |
| 6 | `tlive notify` command | 1 rewrite | 3 |
| 7 | `tlive daemon` commands | 1 rewrite | 3 |
| 8 | Notification relay | 1 modify + 1 new | 5 |
| 9 | Wire full mode | 1 modify + 1 modify | 3 |
| 10 | Integration test + cleanup | 1 new + tidy | 5 |
