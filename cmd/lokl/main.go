package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/supervisor"
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

var upCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "Start the development environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		fmt.Printf("\n  lokl - %s\n\n", cfg.Name)

		sup := supervisor.New(cfg)

		if err := sup.Start(); err != nil {
			return err
		}

		fmt.Println("\n  Press Ctrl+C to stop")
		sup.Wait()

		if err := sup.Stop(); err != nil {
			return err
		}

		return nil
	},
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
	rootCmd.AddCommand(upCmd, downCmd, statusCmd)
}
