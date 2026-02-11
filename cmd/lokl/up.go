package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/docker"
	"github.com/shahin-bayat/lokl/internal/logger"
	"github.com/shahin-bayat/lokl/internal/process"
	"github.com/shahin-bayat/lokl/internal/proxy"
	"github.com/shahin-bayat/lokl/internal/supervisor"
	"github.com/shahin-bayat/lokl/internal/tui"
)

var detach bool

var upCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "Start the development environment",
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVarP(&detach, "detach", "d", false, "run without TUI")
}

func runUp(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	var dockerClient *docker.Client
	for _, svc := range cfg.Services {
		if svc.Image != "" {
			dockerClient, err = docker.NewClient()
			if err != nil {
				return fmt.Errorf("docker required but unavailable: %w", err)
			}
			defer func() { _ = dockerClient.Close() }()
			break
		}
	}

	pf := func(name string, svc config.Service, onChange func()) supervisor.ProcessRunner {
		if svc.Image != "" {
			return docker.NewContainer(name, svc, dockerClient, onChange)
		}
		return process.New(name, svc, onChange)
	}

	log := logger.New(os.Stdout)
	pm := proxy.New(cfg)

	sup := supervisor.New(cfg, pf, pm, log)

	if err := sup.Start(); err != nil {
		var dnsErr *supervisor.DNSNotConfiguredError
		if errors.As(err, &dnsErr) {
			log.Infof("\n⚠ DNS entries needed for: %s\n", strings.Join(dnsErr.Domains, ", "))
			log.Infof("\nOption 1 - Run:\n")
			log.Infof("  sudo lokl dns setup\n")
			log.Infof("\nOption 2 - Add manually to /etc/hosts:\n")
			log.Infof("  %s\n", strings.ReplaceAll(dnsErr.DNSBlock, "\n", "\n  "))
		}
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
}
