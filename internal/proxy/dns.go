package proxy

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const hostsFile = "/etc/hosts"

type HostsManager struct {
	project string
}

func NewHostsManager(project string) *HostsManager {
	return &HostsManager{project: project}
}

func (h *HostsManager) Add(domains []string) error {
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

func (h *HostsManager) Remove() error {
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

func (h *HostsManager) NeedsSudo() bool {
	f, err := os.OpenFile(hostsFile, os.O_WRONLY, 0o644)
	if err != nil {
		return true
	}
	_ = f.Close()
	return false
}

func (h *HostsManager) HasEntries() (bool, error) {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return false, fmt.Errorf("reading hosts file: %w", err)
	}
	return strings.Contains(string(content), h.startMarker()), nil
}

func (h *HostsManager) startMarker() string {
	return fmt.Sprintf("# lokl:%s - START", h.project)
}

func (h *HostsManager) endMarker() string {
	return fmt.Sprintf("# lokl:%s - END", h.project)
}

func (h *HostsManager) removeBlock(content string) string {
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
