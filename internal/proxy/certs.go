package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type CertManager struct {
	dir string
}

func NewCertManager(dir string) *CertManager {
	return &CertManager{dir: dir}
}

func (c *CertManager) EnsureCA() error {
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

func (c *CertManager) Generate(domain string) (certPath, keyPath string, err error) {
	if err := c.checkMkcert(); err != nil {
		return "", "", err
	}

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating cert directory: %w", err)
	}

	certPath = c.CertPath(domain)
	keyPath = c.KeyPath(domain)

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

func (c *CertManager) CertPath(domain string) string {
	return filepath.Join(c.dir, domain+".pem")
}

func (c *CertManager) KeyPath(domain string) string {
	return filepath.Join(c.dir, domain+"-key.pem")
}

func (c *CertManager) checkMkcert() error {
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
