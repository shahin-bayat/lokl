package main

import (
	"fmt"
	"os"

	"github.com/shahin-bayat/devenv/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "devenv",
	Short:   "Development environment orchestrator",
	Long:    "DevEnv - Define and run your local development environment with a single command.",
	Version: version.Version,
}

var upCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "Start the development environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("devenv up - not implemented")
		return nil
	},
}

var downCmd = &cobra.Command{
	Use:   "down [services...]",
	Short: "Stop the development environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("devenv down - not implemented")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"ps"},
	Short:   "Show status of services",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("devenv status - not implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(upCmd, downCmd, statusCmd)
}
