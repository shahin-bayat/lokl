package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type certManager struct {
	dir string
}

func newCertManager(dir string) *certManager {
	return &certManager{dir: dir}
}

func (c *certManager) ensureCA() error {
	if err := c.checkMkcert(); err != nil {
		return err
	}

	out, err := exec.Command("mkcert", "-install").CombinedOutput()
	if err != nil {
		return fmt.Errorf("installing mkcert CA: %s: %w", out, err)
	}

	return nil
}

func (c *certManager) generate(primary string, sans []string) (certPath, keyPath string, err error) {
	if err := c.checkMkcert(); err != nil {
		return "", "", err
	}

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating cert directory: %w", err)
	}

	certPath = c.certPath(primary, sans)
	keyPath = c.keyPath(primary, sans)

	if fileExists(certPath) && fileExists(keyPath) {
		return certPath, keyPath, nil
	}

	args := []string{"-cert-file", certPath, "-key-file", keyPath}
	args = append(args, sans...)

	out, err := exec.Command("mkcert", args...).CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("generating certificate: %s: %w", out, err)
	}

	return certPath, keyPath, nil
}

func (c *certManager) certPath(domain string, sans []string) string {
	return filepath.Join(c.dir, domain+"."+sanHash(sans)+".pem")
}

func (c *certManager) keyPath(domain string, sans []string) string {
	return filepath.Join(c.dir, domain+"."+sanHash(sans)+"-key.pem")
}

// sanHash produces a stable short hash of the SAN set so the cert filename
// changes when the SAN list changes, invalidating the on-disk cache.
func sanHash(sans []string) string {
	uniq := make(map[string]struct{}, len(sans))
	for _, s := range sans {
		uniq[s] = struct{}{}
	}
	sorted := make([]string, 0, len(uniq))
	for s := range uniq {
		sorted = append(sorted, s)
	}
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))
	return hex.EncodeToString(sum[:])[:8]
}

func (c *certManager) checkMkcert() error {
	_, err := exec.LookPath("mkcert")
	if err != nil {
		return fmt.Errorf("mkcert not found: install with 'brew install mkcert' or see https://github.com/FiloSottile/mkcert")
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
