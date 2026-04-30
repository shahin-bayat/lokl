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
	// Missing returns the subset of parents that lack resolver installation.
	Missing(parents []string) []string
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
	// Install resolver files first — the riskier step — so a failure doesn't leave
	// /etc/hosts partially mutated.
	if len(d.wildcardParents) > 0 {
		if err := d.resolver.Write(d.wildcardParents); err != nil {
			return fmt.Errorf("writing resolver files: %w", err)
		}
	}
	if err := d.add(exactDomains); err != nil {
		if len(d.wildcardParents) > 0 {
			_ = d.resolver.Remove(d.wildcardParents)
		}
		return err
	}
	if len(d.wildcardParents) > 0 {
		_ = d.resolver.FlushCache()
	}
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
		fmt.Fprintf(&block, "::1 %s\n", domain)
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

func (d *dnsManager) MissingWildcardParents() []string {
	if len(d.wildcardParents) == 0 {
		return nil
	}
	return d.resolver.Missing(d.wildcardParents)
}

func (d *dnsManager) unresolved(domains []string) []string {
	var missing []string

	// Parse our hosts block once; used as the fallback oracle when LookupHost
	// can't work (see coveredByInstalledWildcard).
	blockHosts := d.currentBlockHosts()

	for _, domain := range domains {
		if d.coveredByInstalledWildcard(domain) {
			if _, ok := blockHosts[domain]; ok {
				continue
			}
			missing = append(missing, domain)
			continue
		}
		if !d.resolvesToLocalhost(domain) {
			missing = append(missing, domain)
		}
	}
	return missing
}

// coveredByInstalledWildcard reports whether domain is under a wildcard parent
// whose resolver file is installed. Such hosts cannot be probed via LookupHost
// on macOS because the resolver file routes the whole zone to our DNS listener,
// which is not running during pre-flight readiness checks.
func (d *dnsManager) coveredByInstalledWildcard(domain string) bool {
	if len(d.wildcardParents) == 0 || d.resolver == nil {
		return false
	}
	missing := map[string]struct{}{}
	for _, p := range d.resolver.Missing(d.wildcardParents) {
		missing[p] = struct{}{}
	}
	for _, parent := range d.wildcardParents {
		if _, isMissing := missing[parent]; isMissing {
			continue
		}
		if domain == parent || strings.HasSuffix(domain, "."+parent) {
			return true
		}
	}
	return false
}

// currentBlockHosts returns the set of hostnames currently present in our
// project's /etc/hosts block.
func (d *dnsManager) currentBlockHosts() map[string]struct{} {
	out := map[string]struct{}{}
	content, err := os.ReadFile(d.hostsPathOrDefault())
	if err != nil {
		return out
	}
	startMarker := d.startMarker()
	endMarker := d.endMarker()

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
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
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		for _, host := range fields[1:] {
			out[host] = struct{}{}
		}
	}
	return out
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
		fmt.Fprintf(&b, "::1 %s\n", domain)
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
