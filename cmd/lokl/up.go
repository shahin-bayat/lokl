package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/docker"
	"github.com/shahin-bayat/lokl/internal/process"
	"github.com/shahin-bayat/lokl/internal/proxy"
	"github.com/shahin-bayat/lokl/internal/supervisor"
	"github.com/shahin-bayat/lokl/internal/tui"
	"github.com/shahin-bayat/lokl/internal/types"
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

	pm := proxy.New(cfg)
	sup := supervisor.New(cfg, pf, pm)

	go printEvents(sup.Subscribe())

	if err := sup.Start(); err != nil {
		return err
	}

	if detach {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		signal.Stop(sigCh)
	} else {
		app := tui.New(sup)
		if err := app.Run(); err != nil {
			_ = sup.Stop()
			return err
		}
	}

	return sup.Stop()
}

func printEvents(ch <-chan types.Event) {
	for ev := range ch {
		switch ev.Type {
		case types.EventProgress:
			if ev.Service != "" {
				fmt.Fprintf(os.Stderr, "✓ %s: %s\n", ev.Service, ev.Message)
			} else {
				fmt.Fprintf(os.Stderr, "✓ %s\n", ev.Message)
			}
		case types.EventError:
			if ev.Service != "" {
				fmt.Fprintf(os.Stderr, "✗ %s: %s\n", ev.Service, ev.Message)
			} else {
				fmt.Fprintf(os.Stderr, "✗ %s\n", ev.Message)
			}
		}
	}
}
