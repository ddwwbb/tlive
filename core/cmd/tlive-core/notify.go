package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/termlive/termlive/core/internal/config"
	"github.com/termlive/termlive/core/internal/daemon"
)

var (
	notifyType    string
	notifyMessage string
	notifyContext string
	notifyQuiet   bool
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
	notifyCmd.Flags().BoolVarP(&notifyQuiet, "quiet", "q", false, "Suppress all output on failure")
}

func runNotify(cmd *cobra.Command, args []string) error {
	if notifyMessage == "" {
		return fmt.Errorf("--message is required")
	}

	// Discover daemon: try lock file first (running daemon), then config file
	daemonPort, token := discoverDaemon()
	if !notifyQuiet {
		log.Printf("notify: type=%s port=%d token_len=%d", notifyType, daemonPort, len(token))
	}

	payload := map[string]string{
		"type":    notifyType,
		"message": notifyMessage,
	}
	if notifyContext != "" {
		payload["context"] = notifyContext
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("http://localhost:%d/api/notify", daemonPort)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Daemon not running — fail silently
		if !notifyQuiet {
			log.Printf("notify: daemon unreachable: %v", err)
			fmt.Fprintln(os.Stderr, "tlive: daemon not running (notification not sent)")
		}
		return nil
	}
	defer resp.Body.Close()

	if !notifyQuiet {
		log.Printf("notify: response status=%d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		if !notifyQuiet {
			fmt.Fprintln(os.Stderr, "tlive: authentication failed — try running 'tlive init' again")
		}
		return nil
	}
	if resp.StatusCode != http.StatusOK && !notifyQuiet {
		fmt.Fprintf(os.Stderr, "tlive: daemon returned status %d\n", resp.StatusCode)
	}
	return nil
}

// discoverDaemon finds the running daemon's port and token.
// Priority: lock file (actual running daemon) > .termlive.toml (project config).
func discoverDaemon() (int, string) {
	// Try lock file first — contains info of actually running daemon
	lock, err := daemon.ReadLockFile(daemon.DefaultLockPath())
	if err == nil && lock.Token != "" {
		return lock.Port, lock.Token
	}

	// Fall back to project config
	cfg, err := config.LoadFromFile(".termlive.toml")
	if err != nil {
		cfg = config.Default()
	}
	return cfg.Daemon.Port, cfg.Daemon.Token
}
