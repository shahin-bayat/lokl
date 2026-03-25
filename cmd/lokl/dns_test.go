package main

import (
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"
)

func TestChownToSudoUser_NoSudoUser(t *testing.T) {
	t.Setenv("SUDO_USER", "")

	dir := t.TempDir()
	// Should be a no-op — no panic, no error.
	chownToSudoUser(dir)
}

func TestChownToSudoUser_UnknownUser(t *testing.T) {
	t.Setenv("SUDO_USER", "this-user-does-not-exist-lokl-test")

	dir := t.TempDir()
	// Should be a no-op — user.Lookup fails silently.
	chownToSudoUser(dir)
}

func TestChownToSudoUser_CurrentUser(t *testing.T) {
	// Set SUDO_USER to the current user — chown to self always succeeds.
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot determine current user")
	}
	t.Setenv("SUDO_USER", u.Username)

	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(subdir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	chownToSudoUser(dir)

	// Verify all paths are owned by current user.
	for _, path := range []string{dir, subdir, file} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		if got := int(stat.Uid); got != mustAtoi(u.Uid) {
			t.Errorf("%s: uid = %d, want %d", path, got, mustAtoi(u.Uid))
		}
	}
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
