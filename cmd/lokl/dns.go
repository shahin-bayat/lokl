package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/proxy"
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Manage DNS entries",
}

var dnsSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Add DNS entries to /etc/hosts",
	RunE:  runDNSSetup,
}

var dnsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove DNS entries from /etc/hosts",
	RunE:  runDNSRemove,
}

func init() {
	dnsCmd.AddCommand(dnsSetupCmd, dnsRemoveCmd)
}

func runDNSSetup(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
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
}

func runDNSRemove(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	p := proxy.New(cfg)

	if err := p.RemoveDNS(); err != nil {
		return fmt.Errorf("removing DNS entries: %w", err)
	}

	fmt.Println("✓ Removed DNS entries from /etc/hosts")
	fmt.Println("\nTo flush DNS cache:")
	if runtime.GOOS == "darwin" {
		fmt.Println("  sudo dscacheutil -flushcache && sudo killall -HUP mDNSResponder")
	} else {
		fmt.Println("  sudo systemd-resolve --flush-caches")
	}
	return nil
}
