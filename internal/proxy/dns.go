package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"time"
)

const (
	hostsFile        = "/etc/hosts"
	dnsLookupTimeout = 2 * time.Second
)

type hostsManager struct {
	project string
}

func newHostsManager(project string) *hostsManager {
	return &hostsManager{project: project}
}

func (h *hostsManager) add(domains []string) error {
	if len(domains) == 0 {
		return nil
	}

	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	cleaned := h.removeBlock(string(content))

	var block strings.Builder
	block.WriteString(h.startMarker() + "\n")
	for _, domain := range domains {
		fmt.Fprintf(&block, "127.0.0.1 %s\n", domain)
	}
	block.WriteString(h.endMarker() + "\n")

	newContent := strings.TrimRight(cleaned, "\n") + "\n\n" + block.String()

	if err := os.WriteFile(hostsFile, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("writing hosts file: %w", err)
	}

	return nil
}

func (h *hostsManager) remove() error {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	cleaned := h.removeBlock(string(content))

	if err := os.WriteFile(hostsFile, []byte(cleaned), 0o644); err != nil {
		return fmt.Errorf("writing hosts file: %w", err)
	}

	return nil
}

func (h *hostsManager) needsSudo() bool {
	f, err := os.OpenFile(hostsFile, os.O_WRONLY, 0o644)
	if err != nil {
		return true
	}
	_ = f.Close()
	return false
}

func (h *hostsManager) unresolved(domains []string) []string {
	var missing []string
	for _, domain := range domains {
		if !h.resolvesToLocalhost(domain) {
			missing = append(missing, domain)
		}
	}
	return missing
}

func (h *hostsManager) resolvesToLocalhost(domain string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(ctx, domain)
	if err != nil {
		return false
	}

	return slices.Contains(addrs, "127.0.0.1") || slices.Contains(addrs, "::1")
}

func (h *hostsManager) block(domains []string) string {
	var b strings.Builder
	b.WriteString(h.startMarker() + "\n")
	for _, domain := range domains {
		fmt.Fprintf(&b, "127.0.0.1 %s\n", domain)
	}
	b.WriteString(h.endMarker())
	return b.String()
}

func (h *hostsManager) startMarker() string {
	return fmt.Sprintf("# lokl:%s - START", h.project)
}

func (h *hostsManager) endMarker() string {
	return fmt.Sprintf("# lokl:%s - END", h.project)
}

func (h *hostsManager) removeBlock(content string) string {
	startMarker := h.startMarker()
	endMarker := h.endMarker()

	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	inBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == startMarker {
			inBlock = true
			continue
		}
		if line == endMarker {
			inBlock = false
			continue
		}
		if !inBlock {
			result.WriteString(line + "\n")
		}
	}

	return result.String()
}
