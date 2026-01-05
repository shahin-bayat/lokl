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

	cmd := exec.Command("mkcert", "-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing mkcert CA: %w", err)
	}

	return nil
}

func (c *certManager) generate(domain string) (certPath, keyPath string, err error) {
	if err := c.checkMkcert(); err != nil {
		return "", "", err
	}

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating cert directory: %w", err)
	}

	certPath = c.certPath(domain)
	keyPath = c.keyPath(domain)

	if fileExists(certPath) && fileExists(keyPath) {
		return certPath, keyPath, nil
	}

	wildcard := "*." + domain
	cmd := exec.Command("mkcert",
		"-cert-file", certPath,
		"-key-file", keyPath,
		wildcard, domain,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("generating certificate: %w", err)
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
