package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var port int
var token string

var rootCmd = &cobra.Command{
	Use:   "tlive",
	Short: "TLive - Terminal live monitoring",
	Long:  "Wrap terminal commands for remote monitoring via Web UI.",
	Args:  cobra.ArbitraryArgs,
	RunE:  runCommand,
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 8080, "Web server port")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "Auth token")
	rootCmd.Flags().StringVar(&publicIP, "ip", "", "Override auto-detected LAN IP address")
	// Allow unknown flags to pass through to wrapped commands (e.g. tlive claude -r)
	rootCmd.FParseErrWhitelist = cobra.FParseErrWhitelist{UnknownFlags: true}
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(installCmd)
}

func setupLogFile() *os.File {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".tlive")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(dir, "tlive.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil
	}
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return f
}

func main() {
	if f := setupLogFile(); f != nil {
		defer f.Close()
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
