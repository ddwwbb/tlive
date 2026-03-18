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
	Short: "TermLive - Terminal live monitoring with AI notifications",
	Long: `TermLive wraps terminal commands for remote monitoring, interaction,
and intelligent notifications via AI tool integration (skills/hooks).`,
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 8080, "Web server / daemon port")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "Authentication token (auto-generated if empty)")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(daemonCmd)
}

func setupLogFile() *os.File {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".termlive")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(dir, "termlive.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
