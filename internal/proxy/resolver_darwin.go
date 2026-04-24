//go:build darwin

package proxy

import (
	"bufio"
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
		owned, err := r.loklOwnedOrAbsent(path)
		if err != nil {
			return err
		}
		if !owned {
			return fmt.Errorf("%s exists and is not managed by lokl; remove it manually and re-run dns setup", path)
		}
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
		owned, err := r.loklOwnedOrAbsent(path)
		if err != nil {
			return err
		}
		if !owned {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}
	return nil
}

// loklOwnedOrAbsent returns true when the file is absent or its first line is our marker.
// Prevents clobbering foreign resolver files set up for VPNs, dnsmasq, etc.
func (r *resolverDir) loklOwnedOrAbsent(path string) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		return scanner.Text() == macResolverMarker, nil
	}
	return true, nil
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
