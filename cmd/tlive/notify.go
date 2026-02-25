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
