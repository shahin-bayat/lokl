package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/docker"
	"github.com/shahin-bayat/lokl/internal/lockfile"
	"github.com/shahin-bayat/lokl/internal/process"
	"github.com/shahin-bayat/lokl/internal/proxy"
	"github.com/shahin-bayat/lokl/internal/supervisor"
	"github.com/shahin-bayat/lokl/internal/tui"
	"github.com/shahin-bayat/lokl/internal/types"
	"github.com/shahin-bayat/lokl/internal/update"
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

	updateCh := make(chan string, 1)
	go func() {
		updateCh <- update.Check(buildVersion)
	}()

	if entry, err := lockfile.Read(cfg.Name); err == nil {
		if !lockfile.IsStale(entry) {
			return fmt.Errorf("lokl is already running for %q (pid %d). Run 'lokl down' to stop it", cfg.Name, entry.PID)
		}
		// Parent died (crash/terminal close) but services may still be running.
		lockfile.KillOrphans(entry)
		_ = lockfile.Remove(cfg.Name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading lock file: %w (delete %s to reset)", err, lockfile.Path(cfg.Name))
	}

	var dockerClient *docker.Client
	var projectNetwork string

	for _, svc := range cfg.Services {
		if svc.Image != "" {
			dockerClient, err = docker.NewClient()
			if err != nil {
				return fmt.Errorf("docker required but unavailable: %w", err)
			}
			defer func() { _ = dockerClient.Close() }()

			projectNetwork = "lokl-" + cfg.Name
			netCtx, netCancel := context.WithTimeout(context.Background(), 30*time.Second)
			netErr := dockerClient.EnsureProjectNetwork(netCtx, projectNetwork)
			netCancel()
			if netErr != nil {
				return fmt.Errorf("creating project network: %w", netErr)
			}
			break // load-bearing: single call guaranteed; idempotency handles crash recovery
		}
	}

	runners := make(map[string]supervisor.ProcessRunner)
	pf := func(name string, svc config.Service, onChange func(), onCrash func()) supervisor.ProcessRunner {
		var r supervisor.ProcessRunner
		if svc.Image != "" {
			r = docker.NewContainer(name, svc, dockerClient, projectNetwork, onChange, onCrash)
		} else {
			r = process.New(name, svc, onChange, onCrash)
		}
		runners[name] = r
		return r
	}

	pm := proxy.New(cfg)
	sup := supervisor.New(cfg, pf, pm)

	if detach {
		go printEvents(sup.Subscribe())
	}

	if err := sup.Start(); err != nil {
		return err
	}

	if err := lockfile.Write(buildLockEntry(cfg.Name, runners)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write lock file: %v\n", err)
	}
	defer func() { _ = lockfile.Remove(cfg.Name) }()
	if projectNetwork != "" {
		defer func() { _ = dockerClient.RemoveNetwork(context.Background(), projectNetwork) }()
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

	result := sup.Stop()

	select {
	case v := <-updateCh:
		if v != "" {
			fmt.Fprintf(os.Stderr, "\nlokl %s is available (you have %s). Run: brew upgrade lokl\n", v, buildVersion)
		}
	case <-time.After(100 * time.Millisecond):
	}

	return result
}

func buildLockEntry(project string, runners map[string]supervisor.ProcessRunner) *lockfile.Entry {
	e := &lockfile.Entry{
		PID:       os.Getpid(),
		Project:   project,
		StartedAt: time.Now().UTC(),
		Processes: make(map[string]int),
	}
	for name, r := range runners {
		switch v := r.(type) {
		case *process.Process:
			if pgid := v.PGID(); pgid != 0 {
				e.Processes[name] = pgid
			}
		case *docker.Container:
			e.Containers = append(e.Containers, "lokl-"+name)
		}
	}
	return e
}

func printEvents(ch <-chan types.Event) {
	for ev := range ch {
		var prefix string
		switch ev.Type {
		case types.EventProgress:
			prefix = "✓"
		case types.EventError:
			prefix = "✗"
		default:
			continue
		}
		if ev.Service != "" {
			fmt.Fprintf(os.Stderr, "%s %s: %s\n", prefix, ev.Service, ev.Message)
		} else {
			fmt.Fprintf(os.Stderr, "%s %s\n", prefix, ev.Message)
		}
	}
}
