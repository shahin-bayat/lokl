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

type resolverWriter interface {
	Write(parents []string) error
	Remove(parents []string) error
	FlushCache() error
}

type dnsManager struct {
	project         string
	wildcardParents []string
	resolver        resolverWriter
	// hostsPath overrides the system hosts file in tests; empty means use hostsFile.
	hostsPath string
}

func newDNSManager(project string, wildcardParents []string, dnsPort int) *dnsManager {
	return &dnsManager{
		project:         project,
		wildcardParents: wildcardParents,
		resolver:        newResolverDir(dnsPort),
	}
}

func (d *dnsManager) hostsPathOrDefault() string {
	if d.hostsPath != "" {
		return d.hostsPath
	}
	return hostsFile
}

func (d *dnsManager) Setup(exactDomains []string) error {
	if err := d.add(exactDomains); err != nil {
		return err
	}
	if len(d.wildcardParents) == 0 {
		return nil
	}
	if err := d.resolver.Write(d.wildcardParents); err != nil {
		return fmt.Errorf("writing resolver files: %w", err)
	}
	_ = d.resolver.FlushCache()
	return nil
}

func (d *dnsManager) Remove() error {
	if err := d.remove(); err != nil {
		return err
	}
	if len(d.wildcardParents) == 0 {
		return nil
	}
	if err := d.resolver.Remove(d.wildcardParents); err != nil {
		return fmt.Errorf("removing resolver files: %w", err)
	}
	_ = d.resolver.FlushCache()
	return nil
}

func (d *dnsManager) add(domains []string) error {
	if len(domains) == 0 {
		return nil
	}

	path := d.hostsPathOrDefault()
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	cleaned := d.removeBlock(string(content))

	var block strings.Builder
	block.WriteString(d.startMarker() + "\n")
	for _, domain := range domains {
		fmt.Fprintf(&block, "127.0.0.1 %s\n", domain)
	}
	block.WriteString(d.endMarker() + "\n")

	newContent := strings.TrimRight(cleaned, "\n") + "\n\n" + block.String()

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("writing hosts file: %w", err)
	}

	return nil
}

func (d *dnsManager) remove() error {
	path := d.hostsPathOrDefault()
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	cleaned := d.removeBlock(string(content))

	if err := os.WriteFile(path, []byte(cleaned), 0o644); err != nil {
		return fmt.Errorf("writing hosts file: %w", err)
	}

	return nil
}

func (d *dnsManager) needsSudo() bool {
	f, err := os.OpenFile(d.hostsPathOrDefault(), os.O_WRONLY, 0o644)
	if err != nil {
		return true
	}
	_ = f.Close()
	return false
}

func (d *dnsManager) unresolved(domains []string) []string {
	var missing []string
	for _, domain := range domains {
		if !d.resolvesToLocalhost(domain) {
			missing = append(missing, domain)
		}
	}
	return missing
}

func (d *dnsManager) resolvesToLocalhost(domain string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(ctx, domain)
	if err != nil {
		return false
	}

	return slices.Contains(addrs, "127.0.0.1") || slices.Contains(addrs, "::1")
}

func (d *dnsManager) block(domains []string) string {
	var b strings.Builder
	b.WriteString(d.startMarker() + "\n")
	for _, domain := range domains {
		fmt.Fprintf(&b, "127.0.0.1 %s\n", domain)
	}
	b.WriteString(d.endMarker())
	return b.String()
}

func (d *dnsManager) startMarker() string {
	return fmt.Sprintf("# lokl:%s - START", d.project)
}

func (d *dnsManager) endMarker() string {
	return fmt.Sprintf("# lokl:%s - END", d.project)
}

func (d *dnsManager) removeBlock(content string) string {
	startMarker := d.startMarker()
	endMarker := d.endMarker()

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
