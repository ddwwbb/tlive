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
	fmt.Fprintln(os.Stderr, "tlive: use Ctrl+C or kill the daemon process to stop it")
	fmt.Fprintln(os.Stderr, "  (PID file based stop will be added in a future version)")
	return nil
}
