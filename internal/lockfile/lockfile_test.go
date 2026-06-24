package lockfile_test

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/shahin-bayat/lokl/internal/lockfile"
)

func sampleEntry(project string) *lockfile.Entry {
	return &lockfile.Entry{
		PID:        os.Getpid(),
		Project:    project,
		StartedAt:  time.Now().UTC().Truncate(time.Second),
		Processes:  map[string]int{"api": 12350, "worker": 12360},
		Containers: []string{"lokl-redis"},
	}
}

func TestPath(t *testing.T) {
	p := lockfile.Path("my-app")
	if p == "" {
		t.Fatal("expected non-empty path")
	}
	if got, want := p, "lokl-my-app.lock"; len(got) < len(want) {
		t.Errorf("path %q too short", got)
	}
}

func TestWriteRead(t *testing.T) {
	project := "test-write-read"
	t.Cleanup(func() { _ = lockfile.Remove(project) })

	in := sampleEntry(project)
	if err := lockfile.Write(in); err != nil {
		t.Fatalf("Write: %v", err)
	}

	out, err := lockfile.Read(project)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if out.PID != in.PID {
		t.Errorf("PID: got %d, want %d", out.PID, in.PID)
	}
	if out.Project != in.Project {
		t.Errorf("Project: got %q, want %q", out.Project, in.Project)
	}
	if len(out.Processes) != len(in.Processes) {
		t.Errorf("Processes len: got %d, want %d", len(out.Processes), len(in.Processes))
	}
	if len(out.Containers) != len(in.Containers) || out.Containers[0] != in.Containers[0] {
		t.Errorf("Containers: got %v, want %v", out.Containers, in.Containers)
	}
}

func TestWriteAtomic(t *testing.T) {
	project := "test-atomic"
	t.Cleanup(func() { _ = lockfile.Remove(project) })

	// Temp file should not persist after Write.
	tmp := lockfile.Path(project) + ".tmp"
	e := sampleEntry(project)
	if err := lockfile.Write(e); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("temp file %q still exists after Write", tmp)
	}
}

func TestRemove(t *testing.T) {
	project := "test-remove"
	e := sampleEntry(project)
	if err := lockfile.Write(e); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := lockfile.Remove(project); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(lockfile.Path(project)); !os.IsNotExist(err) {
		t.Error("file still exists after Remove")
	}
	// Double remove must not error.
	if err := lockfile.Remove(project); err != nil {
		t.Errorf("double Remove: %v", err)
	}
}

func TestIsStale_CurrentPID(t *testing.T) {
	e := &lockfile.Entry{PID: os.Getpid()}
	if lockfile.IsStale(e) {
		t.Error("current process should not be stale")
	}
}

func TestIsStale_MissingPID(t *testing.T) {
	// PID 0 is invalid; Kill(0, 0) targets the current process group.
	// Use a very large PID that almost certainly doesn't exist.
	e := &lockfile.Entry{PID: 999999999}
	if !lockfile.IsStale(e) {
		t.Error("non-existent PID should be stale")
	}
}

// KillOrphans must not return until the process group is actually gone,
// otherwise the caller races the orphan's socket teardown when rebinding.
func TestKillOrphansWaitsForExit(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pgid := cmd.Process.Pid // Setpgid => pgid == pid
	go func() { _, _ = cmd.Process.Wait() }()

	lockfile.KillOrphans(&lockfile.Entry{Processes: map[string]int{"svc": pgid}})

	// Immediately after KillOrphans returns, the group must be gone.
	if err := syscall.Kill(-pgid, 0); err == nil {
		t.Fatal("process group still alive after KillOrphans returned")
	}
}

func TestReadMissing(t *testing.T) {
	_, err := lockfile.Read("no-such-project-xyzzy")
	if err == nil {
		t.Error("expected error reading missing lock file")
	}
}

func TestReadCorrupt(t *testing.T) {
	project := "test-corrupt"
	t.Cleanup(func() { _ = lockfile.Remove(project) })

	if err := os.WriteFile(lockfile.Path(project), []byte("not-json{{"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := lockfile.Read(project)
	if err == nil {
		t.Error("expected error on corrupt JSON")
	}
}
