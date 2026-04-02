// Package lockfile manages a per-project lock file for orphan process recovery.
package lockfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/shahin-bayat/lokl/internal/runner"
)

const killGracePeriod = 5 * time.Second

type Entry struct {
	PID        int            `json:"pid"`
	Project    string         `json:"project"`
	StartedAt  time.Time      `json:"started_at"`
	Processes  map[string]int `json:"processes"`  // service name → PGID
	Containers []string       `json:"containers"` // Docker container names
}

func Path(project string) string {
	return filepath.Join(os.TempDir(), "lokl-"+project+".lock")
}

func Write(e *Entry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshaling lock entry: %w", err)
	}

	path := Path(e.Project)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("installing lock file: %w", err)
	}
	return nil
}

func Read(project string) (*Entry, error) {
	data, err := os.ReadFile(Path(project))
	if err != nil {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parsing lock file: %w", err)
	}
	return &e, nil
}

func Remove(project string) error {
	err := os.Remove(Path(project))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func IsStale(e *Entry) bool {
	if e.PID <= 0 {
		return true
	}
	err := syscall.Kill(e.PID, 0)
	return errors.Is(err, syscall.ESRCH)
}

// KillOrphans SIGKILLs services from a stale lock (parent already dead).
func KillOrphans(e *Entry) {
	signalProcesses(e, syscall.SIGKILL)
	stopContainers(e)
}

// Kill sends SIGTERM to the parent lokl process and all service PGIDs,
// then SIGKILLs survivors after the grace period.
func Kill(e *Entry) {
	if e.PID > 0 {
		_ = syscall.Kill(e.PID, syscall.SIGTERM)
	}
	signalProcesses(e, syscall.SIGTERM)

	if e.PID > 0 || len(e.Processes) > 0 {
		time.Sleep(killGracePeriod)
		signalProcesses(e, syscall.SIGKILL)
		if e.PID > 0 {
			_ = syscall.Kill(e.PID, syscall.SIGKILL)
		}
	}

	stopContainers(e)
}

func signalProcesses(e *Entry, sig syscall.Signal) {
	for _, pgid := range e.Processes {
		if pgid <= 0 {
			continue
		}
		runner.SignalTree(pgid, sig)
		_ = syscall.Kill(-pgid, sig)
	}
}

func stopContainers(e *Entry) {
	for _, name := range e.Containers {
		_ = exec.Command("docker", "stop", name).Run()
		_ = exec.Command("docker", "rm", name).Run()
	}
}
