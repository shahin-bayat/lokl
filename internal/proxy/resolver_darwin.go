//go:build darwin

package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	macResolverDir    = "/etc/resolver"
	macResolverMarker = "# managed by lokl"
)

type resolverDir struct {
	base string
	port int
}

func newResolverDir(port int) *resolverDir {
	return &resolverDir{base: macResolverDir, port: port}
}

func (r *resolverDir) Write(parents []string) error {
	if err := os.MkdirAll(r.base, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", r.base, err)
	}
	for _, p := range parents {
		path := filepath.Join(r.base, p)
		content := fmt.Sprintf("%s\nnameserver 127.0.0.1\nport %d\n", macResolverMarker, r.port)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func (r *resolverDir) Remove(parents []string) error {
	for _, p := range parents {
		path := filepath.Join(r.base, p)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}
	return nil
}

func (r *resolverDir) Missing(parents []string) []string {
	var missing []string
	for _, p := range parents {
		if _, err := os.Stat(filepath.Join(r.base, p)); err != nil {
			missing = append(missing, p)
		}
	}
	return missing
}

// FlushCache is best-effort; a stale cache is a minor UX issue, not a correctness one.
func (r *resolverDir) FlushCache() error {
	_ = exec.Command("dscacheutil", "-flushcache").Run()
	_ = exec.Command("killall", "-HUP", "mDNSResponder").Run()
	return nil
}
