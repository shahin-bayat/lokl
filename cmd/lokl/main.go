package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/version"
)

const defaultConfigFile = "lokl.yaml"

var configFile string

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "lokl",
	Short:   "Local development environment orchestrator",
	Long:    "lokl - Define and run your local development environment with a single command.",
	Version: version.Version,
}

var downCmd = &cobra.Command{
	Use:   "down [services...]",
	Short: "Stop the development environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Running in foreground mode. Use Ctrl+C to stop.")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"ps"},
	Short:   "Show status of services",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Running in foreground mode. No daemon to query.")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", defaultConfigFile, "config file path")
	rootCmd.AddCommand(upCmd, downCmd, statusCmd, dnsCmd, initCmd)
}

func waitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
