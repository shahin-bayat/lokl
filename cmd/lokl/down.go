package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/docker"
	"github.com/shahin-bayat/lokl/internal/lockfile"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the development environment",
	RunE:  runDown,
}

func runDown(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	entry, err := lockfile.Read(cfg.Name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("lokl is not running")
			return nil
		}
		return fmt.Errorf("reading lock file: %w (delete %s to reset)", err, lockfile.Path(cfg.Name))
	}

	fmt.Printf("stopping %d service(s)...\n", len(entry.Processes)+len(entry.Containers))
	lockfile.Kill(entry)

	if len(entry.Containers) > 0 {
		if dc, err := docker.NewClient(); err == nil {
			_ = dc.RemoveNetwork(context.Background(), "lokl-"+entry.Project)
			_ = dc.Close()
		}
	}

	return lockfile.Remove(cfg.Name)
}
