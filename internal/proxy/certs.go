package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	certPath = c.certPath(primary)
	keyPath = c.keyPath(primary)

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

func (c *certManager) certPath(domain string) string {
	return filepath.Join(c.dir, domain+".pem")
}

func (c *certManager) keyPath(domain string) string {
	return filepath.Join(c.dir, domain+"-key.pem")
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
