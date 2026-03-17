package daemon

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

// GitStatus holds the result of a git status check.
type GitStatus struct {
	Branch    string `json:"branch"`
	DiffCount int    `json:"diff_count"`
	Clean     bool   `json:"clean"`
	Error     string `json:"error,omitempty"`
}

// GetGitStatus returns the git status for the given working directory.
// If the directory is not a git repository or git is not available,
// it returns a GitStatus with the Error field set to "not a git repository".
func GetGitStatus(workdir string) GitStatus {
	// Get the current branch name
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = workdir
	branchOut, err := branchCmd.Output()
	if err != nil {
		return GitStatus{Error: "not a git repository"}
	}

	branch := strings.TrimSpace(string(branchOut))

	// Get the porcelain status for clean check and diff count
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = workdir
	statusOut, err := statusCmd.Output()
	if err != nil {
		return GitStatus{Error: "not a git repository"}
	}

	lines := strings.Split(strings.TrimSpace(string(statusOut)), "\n")
	diffCount := 0
	if strings.TrimSpace(string(statusOut)) != "" {
		diffCount = len(lines)
	}
	clean := diffCount == 0

	return GitStatus{
		Branch:    branch,
		DiffCount: diffCount,
		Clean:     clean,
	}
}

func (d *Daemon) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := GetGitStatus(".")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
