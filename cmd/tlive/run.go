package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	qrterminal "github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"
	"github.com/termlive/termlive/internal/config"
	"github.com/termlive/termlive/internal/daemon"
	"github.com/termlive/termlive/internal/notify"
	"github.com/termlive/termlive/internal/server"
	"github.com/termlive/termlive/web"
	"golang.org/x/term"
)

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

// localOutputClient implements hub.Client to write PTY output to local
// stdout and feed the idle detector. It is registered on the session hub
// so that the SessionManager's output goroutine delivers data here.
type localOutputClient struct {
	writer       *os.File
	idleDetector *notify.SmartIdleDetector
}

func (c *localOutputClient) Send(data []byte) error {
	c.writer.Write(data)
	if c.idleDetector != nil {
		c.idleDetector.Feed(data)
	}
	return nil
}

func runCommand(cmd *cobra.Command, args []string) error {
	cfg := config.Default()
	cfg.Server.Port = port
	cfg.Notify.ShortTimeout = shortTimeout
	cfg.Notify.LongTimeout = longTimeout

	rows, cols := uint16(24), uint16(80)
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		cols, rows = uint16(w), uint16(h)
	}

	lockPath := daemon.DefaultLockPath()

	// --- Determine host vs client mode ---
	isHost := true
	lock, err := daemon.ReadLockFile(lockPath)
	if err == nil && daemonHealthCheck(lock.Port, lock.Token) {
		isHost = false
	}

	if isHost {
		return runHost(cfg, args, rows, cols, lockPath)
	}
	return runClient(lock, args, rows, cols)
}

// runHost starts an embedded daemon (first process) and runs the command
// directly via the in-process SessionManager.
func runHost(cfg *config.Config, args []string, rows, cols uint16, lockPath string) error {
	// Create daemon
	d := daemon.NewDaemon(daemon.DaemonConfig{
		Port:         cfg.Server.Port,
		HistoryLimit: cfg.Notify.Options.HistoryLimit,
	})
	mgr := d.Manager()

	// Create session directly (in-process, no HTTP)
	ms, err := mgr.CreateSession(args[0], args[1:], daemon.SessionConfig{
		Rows: rows, Cols: cols,
	})
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}
	defer mgr.StopSession(ms.Session.ID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup notifiers
	var notifiers []notify.Notifier
	if cfg.Notify.WeChat.WebhookURL != "" {
		notifiers = append(notifiers, notify.NewWeChatNotifier(cfg.Notify.WeChat.WebhookURL))
	}
	if cfg.Notify.Feishu.WebhookURL != "" {
		notifiers = append(notifiers, notify.NewFeishuNotifier(cfg.Notify.Feishu.WebhookURL))
	}
	multiNotifier := notify.NewMultiNotifier(notifiers...)
	d.SetNotifiers(multiNotifier)

	// Setup smart idle detector
	localIP := publicIP
	if localIP == "" {
		localIP = getLocalIP()
	}
	idleDetector := notify.NewSmartIdleDetector(
		time.Duration(cfg.Notify.ShortTimeout)*time.Second,
		time.Duration(cfg.Notify.LongTimeout)*time.Second,
		cfg.Notify.Patterns.AwaitingInput,
		cfg.Notify.Patterns.Processing,
		func(confidence string) {
			msg := &notify.NotifyMessage{
				SessionID:   ms.Session.ID,
				Command:     ms.Session.Command,
				Pid:         ms.Session.Pid,
				Duration:    ms.Session.Duration().Truncate(time.Second).String(),
				LastOutput:  string(ms.Session.LastOutput(200)),
				WebURL:      fmt.Sprintf("http://%s:%d/terminal.html?id=%s", localIP, cfg.Server.Port, ms.Session.ID),
				IdleSeconds: cfg.Notify.ShortTimeout,
				Confidence:  confidence,
			}
			if confidence == "low" {
				msg.IdleSeconds = cfg.Notify.LongTimeout
			}
			if err := multiNotifier.Send(msg); err != nil {
				log.Printf("notification error: %v", err)
			}
		},
	)
	idleDetector.Start()

	// Register local output client on the hub so that PTY output is
	// written to local stdout and fed to the idle detector.
	localClient := &localOutputClient{
		writer:       os.Stdout,
		idleDetector: idleDetector,
	}
	ms.Hub.Register(localClient)
	defer ms.Hub.Unregister(localClient)

	// Setup Web UI server as extra handler on the daemon.
	srv := server.New(mgr)
	mgr.SetResizeFunc(ms.Session.ID, func(r, c uint16) {
		ms.Proc.Resize(r, c)
	})
	srv.SetWebFS(web.Assets)
	d.SetExtraHandler(srv.Handler())

	// Write lock file BEFORE starting listener so clients can discover us
	daemon.WriteLockFile(lockPath, daemon.LockInfo{
		Port:  cfg.Server.Port,
		Token: d.Token(),
		Pid:   os.Getpid(),
	})
	defer daemon.RemoveLockFile(lockPath)

	// Print connection info
	url := fmt.Sprintf("http://%s:%d?token=%s", localIP, cfg.Server.Port, d.Token())
	localURL := fmt.Sprintf("http://localhost:%d?token=%s", cfg.Server.Port, d.Token())
	fmt.Fprintf(os.Stderr, "\n  TermLive Web UI:\n")
	fmt.Fprintf(os.Stderr, "    Local:   %s\n", localURL)
	fmt.Fprintf(os.Stderr, "    Network: %s\n", url)
	fmt.Fprintf(os.Stderr, "  Session: %s (ID: %s)\n\n", ms.Session.Command, ms.Session.ID)
	qrterminal.GenerateHalfBlock(url, qrterminal.L, os.Stderr)
	fmt.Fprintln(os.Stderr)

	// Set terminal to raw mode for proper input pass-through
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	rawMode := err == nil

	// Local terminal input -> PTY (exits when ctx cancelled or stdin errors)
	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				ms.Proc.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Start daemon in goroutine
	go d.Run()

	// Wait for process exit or signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	doneCh := make(chan int, 1)
	go func() {
		doneCh <- ms.ExitCode()
	}()

	var exitCode int
	select {
	case exitCode = <-doneCh:
		fmt.Fprintf(os.Stderr, "\n  Process exited with code %d\n", exitCode)
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "\n  Received signal: %v\n", sig)
		ms.Proc.Kill()
		exitCode = 130
	}

	// Cleanup: cancel context first to signal all goroutines
	cancel()

	// Restore terminal BEFORE other cleanup (critical for Windows forced close)
	if rawMode {
		term.Restore(int(os.Stdin.Fd()), oldState)
	}

	// Stop idle detector
	idleDetector.Stop()

	// Check for remaining sessions (from client mode processes).
	// StopSession is deferred, so other sessions from clients may still be active.
	// We need to wait for them before shutting down the daemon.
	if mgr.ActiveCount() > 1 { // >1 because our session hasn't been stopped yet (deferred)
		fmt.Fprintf(os.Stderr, "  Daemon still serving %d other session(s). Press Ctrl+C to stop.\n", mgr.ActiveCount()-1)
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
		sigCh2 := make(chan os.Signal, 1)
		signal.Notify(sigCh2, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh2
		fmt.Fprintf(os.Stderr, "  Shutting down daemon...\n")
	}

	// deferred: mgr.StopSession, RemoveLockFile
	d.Stop()

	_ = exitCode
	return nil
}

// runClient connects to an already-running daemon and creates a new session
// via HTTP API, then relays I/O over WebSocket.
func runClient(lock daemon.LockInfo, args []string, rows, cols uint16) error {
	// Create session via HTTP API
	sessionID, err := createSessionViaAPI(lock.Port, lock.Token, args[0], args[1:], rows, cols)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer deleteSessionViaAPI(lock.Port, lock.Token, sessionID)

	fmt.Fprintf(os.Stderr, "\n  TermLive (client mode):\n")
	fmt.Fprintf(os.Stderr, "    Daemon:  http://localhost:%d\n", lock.Port)
	fmt.Fprintf(os.Stderr, "    Session: %s (ID: %s)\n\n", args[0], sessionID)

	// Set terminal to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	rawMode := err == nil
	defer func() {
		if rawMode {
			term.Restore(int(os.Stdin.Fd()), oldState)
		}
	}()

	// Connect WebSocket
	wsURL := fmt.Sprintf("ws://localhost:%d/ws/%s", lock.Port, sessionID)
	header := http.Header{}
	header.Set("Cookie", fmt.Sprintf("tl_token=%s", lock.Token))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if rawMode {
			term.Restore(int(os.Stdin.Fd()), oldState)
		}
		return fmt.Errorf("websocket connect: %w", err)
	}
	defer conn.Close()

	// Send initial resize
	resizeMsg, _ := json.Marshal(map[string]interface{}{
		"type": "resize",
		"rows": rows,
		"cols": cols,
	})
	conn.WriteMessage(websocket.TextMessage, resizeMsg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// WS -> stdout
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
			os.Stdout.Write(msg)
		}
	}()

	// stdin -> WS
	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Wait for context cancellation (from WS close, signal, or stdin error)
	<-ctx.Done()

	fmt.Fprintf(os.Stderr, "\n  Session ended.\n")
	return nil
}

// --- Helper functions ---

// daemonHealthCheck pings the daemon status endpoint to verify it is alive.
func daemonHealthCheck(port int, token string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://localhost:%d/api/status", port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// createSessionViaAPI creates a new session on the remote daemon via HTTP POST.
func createSessionViaAPI(port int, token string, command string, args []string, rows, cols uint16) (string, error) {
	reqBody := daemon.CreateSessionRequest{
		Command: command,
		Args:    args,
		Rows:    rows,
		Cols:    cols,
	}
	data, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("http://localhost:%d/api/sessions", port)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	var result daemon.CreateSessionResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return result.ID, nil
}

// deleteSessionViaAPI deletes a session on the remote daemon via HTTP DELETE.
func deleteSessionViaAPI(port int, token string, sessionID string) {
	url := fmt.Sprintf("http://localhost:%d/api/sessions/%s", port, sessionID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// isPrivateIP reports whether ip is an RFC 1918 private address
// (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16). This filters out
// VPN/tunnel adapters (e.g. Cloudflare WARP 198.18.0.0/15) that
// are not reachable from the local network.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		network *net.IPNet
	}{
		{parseCIDR("10.0.0.0/8")},
		{parseCIDR("172.16.0.0/12")},
		{parseCIDR("192.168.0.0/16")},
	}
	for _, r := range privateRanges {
		if r.network.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCIDR(s string) *net.IPNet {
	_, network, _ := net.ParseCIDR(s)
	return network
}

func getLocalIP() string {
	// UDP dial trick: connect to a public IP to find the preferred outbound interface.
	// No actual traffic is sent since UDP is connectionless.
	conn, err := net.DialTimeout("udp4", "8.8.8.8:53", 1*time.Second)
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP.To4() != nil && !addr.IP.IsLoopback() && isPrivateIP(addr.IP) {
			return addr.IP.String()
		}
	}

	// Fallback: iterate interfaces, prefer private (RFC 1918) IPv4 addresses.
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil && isPrivateIP(ipnet.IP) {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}
