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
