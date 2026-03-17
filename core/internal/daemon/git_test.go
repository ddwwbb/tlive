package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetGitStatus_InRepo(t *testing.T) {
	// Use the project root directory which is a git repo
	status := GetGitStatus("/home/y/Project/test/TermLive")

	if status.Error != "" {
		t.Fatalf("expected no error in git repo, got: %s", status.Error)
	}
	if status.Branch == "" {
		t.Error("expected non-empty branch name")
	}
	// DiffCount should be >= 0
	if status.DiffCount < 0 {
		t.Errorf("expected diff_count >= 0, got %d", status.DiffCount)
	}
	// Clean should be a valid bool (just verify field exists by using it)
	_ = status.Clean
}

func TestGetGitStatus_NotRepo(t *testing.T) {
	status := GetGitStatus("/tmp")

	if status.Error == "" {
		t.Fatal("expected error for non-git directory, got none")
	}
	if status.Error != "not a git repository" {
		t.Errorf("expected error 'not a git repository', got: %s", status.Error)
	}
}

func TestGitStatusAPI(t *testing.T) {
	d := NewDaemon(DaemonConfig{Port: 0, Token: "test-token"})
	handler := d.Handler()

	req := httptest.NewRequest("GET", "/api/git/status", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp GitStatus
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	// The daemon runs in some directory; we just verify the response is valid JSON
	// with expected fields (branch may or may not be set depending on working dir)
	_ = resp.Branch
	_ = resp.DiffCount
	_ = resp.Clean
}
