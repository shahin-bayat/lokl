//go:build darwin

package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolverFileLifecycle(t *testing.T) {
	dir := t.TempDir()
	r := &resolverDir{base: dir, port: 5454}

	if err := r.Write([]string{"sellify.shop", "sellify.dev"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, parent := range []string{"sellify.shop", "sellify.dev"} {
		b, err := os.ReadFile(filepath.Join(dir, parent))
		if err != nil {
			t.Fatalf("read %s: %v", parent, err)
		}
		content := string(b)
		if !strings.Contains(content, "nameserver 127.0.0.1") {
			t.Fatalf("missing nameserver line in %s:\n%s", parent, content)
		}
		if !strings.Contains(content, "port 5454") {
			t.Fatalf("missing port line in %s:\n%s", parent, content)
		}
	}

	// Idempotent second write.
	if err := r.Write([]string{"sellify.shop", "sellify.dev"}); err != nil {
		t.Fatalf("write again: %v", err)
	}

	// Removing one parent leaves the other intact.
	if err := r.Remove([]string{"sellify.shop"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sellify.shop")); !os.IsNotExist(err) {
		t.Fatalf("expected sellify.shop removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sellify.dev")); err != nil {
		t.Fatalf("sellify.dev should still exist: %v", err)
	}

	// Remove is idempotent (no error when file is already gone).
	if err := r.Remove([]string{"sellify.shop"}); err != nil {
		t.Fatalf("remove idempotent: %v", err)
	}
}

func TestResolverFileRespectsForeignFiles(t *testing.T) {
	dir := t.TempDir()
	r := &resolverDir{base: dir, port: 5454}

	foreign := filepath.Join(dir, "foreign.test")
	if err := os.WriteFile(foreign, []byte("nameserver 9.9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := r.Write([]string{"foreign.test"}); err == nil {
		t.Fatal("Write should refuse to overwrite a foreign resolver file")
	}
	if err := r.Remove([]string{"foreign.test"}); err != nil {
		t.Fatalf("Remove should not error on foreign file: %v", err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign file should survive Remove: %v", err)
	}
}

func TestResolverFileOverwritesLoklOwned(t *testing.T) {
	dir := t.TempDir()
	r := &resolverDir{base: dir, port: 5454}

	if err := r.Write([]string{"sellify.shop"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Write([]string{"sellify.shop"}); err != nil {
		t.Fatalf("second Write on lokl-owned file should succeed: %v", err)
	}
}
