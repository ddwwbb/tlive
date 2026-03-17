package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/termlive/termlive/core/internal/config"
	"github.com/termlive/termlive/core/internal/daemon"
	"github.com/termlive/termlive/core/internal/notify"
	"github.com/termlive/termlive/core/internal/server"
	"github.com/termlive/termlive/core/web"
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

	// Setup external notification channels from config
	var notifiers []notify.Notifier
	if cfg.Notify.WeChat.WebhookURL != "" {
		notifiers = append(notifiers, notify.NewWeChatNotifier(cfg.Notify.WeChat.WebhookURL))
		log.Printf("daemon: wechat notifier enabled")
	}
	if cfg.Notify.Feishu.WebhookURL != "" {
		notifiers = append(notifiers, notify.NewFeishuNotifier(cfg.Notify.Feishu.WebhookURL, cfg.Notify.Feishu.Secret))
		log.Printf("daemon: feishu notifier enabled (signed=%v)", cfg.Notify.Feishu.Secret != "")
	}
	if len(notifiers) > 0 {
		d.SetNotifiers(notify.NewMultiNotifier(notifiers...))
	} else {
		log.Printf("daemon: no external notifiers configured")
	}

	// Setup Web UI + WebSocket handler so clients can connect
	srv := server.New(d.Manager())
	srv.SetWebFS(web.Assets)
	d.SetExtraHandler(srv.Handler())

	// Write lock file so other commands can discover this daemon
	lockPath := daemon.DefaultLockPath()
	daemon.WriteLockFile(lockPath, daemon.LockInfo{
		Port:  daemonPort,
		Token: d.Token(),
		Pid:   os.Getpid(),
	})
	log.Printf("daemon start: port=%d pid=%d lock=%s", daemonPort, os.Getpid(), lockPath)

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
		daemon.RemoveLockFile(lockPath)
		d.Stop()
	}()

	return d.Run()
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(os.Stderr, "tlive: use Ctrl+C or kill the daemon process to stop it")
	fmt.Fprintln(os.Stderr, "  (PID file based stop will be added in a future version)")
	return nil
}
