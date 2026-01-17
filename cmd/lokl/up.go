package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
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

	processFactory := func(name string, svc config.Service, onChange func()) supervisor.ProcessRunner {
		return process.New(name, svc, onChange)
	}

	log := logger.New(os.Stdout)
	sup := supervisor.New(cfg, processFactory, proxy.New(cfg), log)

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
}
