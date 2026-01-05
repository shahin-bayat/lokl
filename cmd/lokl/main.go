package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/process"
	"github.com/shahin-bayat/lokl/internal/proxy"
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

		newProcess := func(name string, svc config.Service) supervisor.ProcessRunner {
			return process.New(name, svc)
		}

		fmt.Printf("\n  lokl - %s\n\n", cfg.Name)

		sup := supervisor.New(cfg, newProcess, proxy.New(cfg))

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

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Manage DNS entries",
}

var dnsSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Add DNS entries to /etc/hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		p := proxy.New(cfg)
		domains := p.Domains()

		if len(domains) == 0 {
			fmt.Println("No domains configured")
			return nil
		}

		if err := p.SetupDNS(); err != nil {
			return fmt.Errorf("adding DNS entries: %w", err)
		}

		fmt.Printf("✓ Added %d entries to /etc/hosts\n", len(domains))
		return nil
	},
}

var dnsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove DNS entries from /etc/hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		p := proxy.New(cfg)

		if err := p.RemoveDNS(); err != nil {
			return fmt.Errorf("removing DNS entries: %w", err)
		}

		fmt.Println("✓ Removed DNS entries from /etc/hosts")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", defaultConfigFile, "config file path")
	dnsCmd.AddCommand(dnsSetupCmd, dnsRemoveCmd)
	rootCmd.AddCommand(upCmd, downCmd, statusCmd, dnsCmd)
}
