package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/logger"
	"github.com/shahin-bayat/lokl/internal/process"
	"github.com/shahin-bayat/lokl/internal/proxy"
	"github.com/shahin-bayat/lokl/internal/supervisor"
	"github.com/shahin-bayat/lokl/internal/tui"
	"github.com/shahin-bayat/lokl/internal/version"
)

const defaultConfigFile = "lokl.yaml"

var (
	configFile string
	detach     bool
)

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

		log := logger.New(os.Stdout)
		sup := supervisor.New(cfg, newProcess, proxy.New(cfg), log)

		if err := sup.Start(); err != nil {
			return err
		}

		if detach {
			log.Infof("\nPress Ctrl+C to stop\n")
			waitForSignal()
			log.Infof("\nShutting down...\n")
		} else {
			app := tui.New(sup)
			if err := app.Run(); err != nil {
				_ = sup.Stop()
				return err
			}
		}

		return sup.Stop()
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
	upCmd.Flags().BoolVarP(&detach, "detach", "d", false, "run without TUI")
	dnsCmd.AddCommand(dnsSetupCmd, dnsRemoveCmd)
	rootCmd.AddCommand(upCmd, downCmd, statusCmd, dnsCmd)
}

func waitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
