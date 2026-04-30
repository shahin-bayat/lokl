package proxy

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type fakeResolver struct {
	written [][]string
	removed [][]string
	flushed int
	missing []string
}

func (f *fakeResolver) Write(p []string) error {
	f.written = append(f.written, append([]string(nil), p...))
	return nil
}
func (f *fakeResolver) Remove(p []string) error {
	f.removed = append(f.removed, append([]string(nil), p...))
	return nil
}
func (f *fakeResolver) FlushCache() error { f.flushed++; return nil }
func (f *fakeResolver) Missing(parents []string) []string {
	var out []string
	for _, p := range parents {
		for _, m := range f.missing {
			if p == m {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

func TestDNSManagerSetupWritesResolverFiles(t *testing.T) {
	tmp := t.TempDir()
	hostsPath := filepath.Join(tmp, "hosts")
	if err := os.WriteFile(hostsPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	fr := &fakeResolver{}
	d := &dnsManager{
		project:         "demo",
		hostsPath:       hostsPath,
		resolver:        fr,
		wildcardParents: []string{"sellify.shop"},
	}
	if err := d.Setup([]string{"api.sellify.shop"}); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if len(fr.written) != 1 || !slices.Equal(fr.written[0], []string{"sellify.shop"}) {
		t.Fatalf("resolver writes=%v", fr.written)
	}
	if fr.flushed != 1 {
		t.Fatalf("flushed=%d want 1", fr.flushed)
	}

	if err := d.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(fr.removed) != 1 || !slices.Equal(fr.removed[0], []string{"sellify.shop"}) {
		t.Fatalf("resolver removes=%v", fr.removed)
	}
	if fr.flushed != 2 {
		t.Fatalf("flushed=%d want 2", fr.flushed)
	}
}

func TestDNSManagerSetupSkipsResolverWithoutWildcards(t *testing.T) {
	tmp := t.TempDir()
	hostsPath := filepath.Join(tmp, "hosts")
	if err := os.WriteFile(hostsPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	fr := &fakeResolver{}
	d := &dnsManager{
		project:   "demo",
		hostsPath: hostsPath,
		resolver:  fr,
	}
	if err := d.Setup([]string{"a.test"}); err != nil {
		t.Fatal(err)
	}
	if len(fr.written) != 0 || fr.flushed != 0 {
		t.Fatalf("resolver should be untouched when no wildcards; writes=%v flushed=%d", fr.written, fr.flushed)
	}
}

func TestUnresolvedReportsMissingWildcardParents(t *testing.T) {
	fr := &fakeResolver{missing: []string{"sellify.shop"}}
	d := &dnsManager{
		project:         "demo",
		wildcardParents: []string{"sellify.shop"},
		resolver:        fr,
	}
	got := d.MissingWildcardParents()
	if len(got) != 1 || got[0] != "sellify.shop" {
		t.Fatalf("got=%v want [sellify.shop]", got)
	}
}

func TestMissingWildcardParentsEmptyWithoutWildcards(t *testing.T) {
	fr := &fakeResolver{}
	d := &dnsManager{project: "demo", resolver: fr}
	if got := d.MissingWildcardParents(); len(got) != 0 {
		t.Fatalf("got=%v want nil", got)
	}
}

type failingResolver struct{ fakeResolver }

func (f *failingResolver) Write(p []string) error { return errors.New("boom") }

func TestSetupDoesNotWriteHostsWhenResolverFails(t *testing.T) {
	tmp := t.TempDir()
	hostsPath := filepath.Join(tmp, "hosts")
	if err := os.WriteFile(hostsPath, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &dnsManager{
		project:         "demo",
		hostsPath:       hostsPath,
		resolver:        &failingResolver{},
		wildcardParents: []string{"sellify.shop"},
	}
	if err := d.Setup([]string{"api.test"}); err == nil {
		t.Fatal("Setup must error when resolver fails")
	}
	b, _ := os.ReadFile(hostsPath)
	if string(b) != "original\n" {
		t.Fatalf("hosts should be untouched when resolver fails; got %q", b)
	}
}

func TestUnresolvedTrustsHostsBlockWhenWildcardResolverInstalled(t *testing.T) {
	tmp := t.TempDir()
	hostsPath := filepath.Join(tmp, "hosts")
	seed := `# lokl:demo - START
127.0.0.1 sellify.shop
127.0.0.1 s3.sellify.shop
# lokl:demo - END
`
	if err := os.WriteFile(hostsPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &dnsManager{
		project:         "demo",
		hostsPath:       hostsPath,
		wildcardParents: []string{"sellify.shop"},
		resolver:        &fakeResolver{missing: nil},
	}
	got := d.unresolved([]string{"sellify.shop", "s3.sellify.shop"})
	if len(got) != 0 {
		t.Fatalf("unresolved should be empty when hosts block has entries and resolver installed; got %v", got)
	}
}

func TestUnresolvedReportsHostsMissingUnderInstalledWildcard(t *testing.T) {
	tmp := t.TempDir()
	hostsPath := filepath.Join(tmp, "hosts")
	if err := os.WriteFile(hostsPath, []byte("# lokl:demo - START\n# lokl:demo - END\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &dnsManager{
		project:         "demo",
		hostsPath:       hostsPath,
		wildcardParents: []string{"sellify.shop"},
		resolver:        &fakeResolver{missing: nil},
	}
	got := d.unresolved([]string{"sellify.shop"})
	if len(got) != 1 || got[0] != "sellify.shop" {
		t.Fatalf("want [sellify.shop], got %v", got)
	}
}

func TestHostsManagerRemoveBlock(t *testing.T) {
	h := newDNSManager("myproject", nil, 0)

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "removes block",
			content: `127.0.0.1 localhost
# lokl:myproject - START
127.0.0.1 app.example.com
127.0.0.1 api.example.com
# lokl:myproject - END
127.0.0.1 other.host
`,
			want: `127.0.0.1 localhost
127.0.0.1 other.host
`,
		},
		{
			name: "no block present",
			content: `127.0.0.1 localhost
127.0.0.1 other.host
`,
			want: `127.0.0.1 localhost
127.0.0.1 other.host
`,
		},
		{
			name: "block at end",
			content: `127.0.0.1 localhost
# lokl:myproject - START
127.0.0.1 app.example.com
# lokl:myproject - END
`,
			want: `127.0.0.1 localhost
`,
		},
		{
			name: "block at start",
			content: `# lokl:myproject - START
127.0.0.1 app.example.com
# lokl:myproject - END
127.0.0.1 localhost
`,
			want: `127.0.0.1 localhost
`,
		},
		{
			name: "different project unchanged",
			content: `# lokl:otherproject - START
127.0.0.1 other.example.com
# lokl:otherproject - END
`,
			want: `# lokl:otherproject - START
127.0.0.1 other.example.com
# lokl:otherproject - END
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.removeBlock(tt.content)
			if got != tt.want {
				t.Errorf("removeBlock():\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestHostsManagerBlock(t *testing.T) {
	h := newDNSManager("myproject", nil, 0)

	got := h.block([]string{"app.example.com", "api.example.com"})

	want := `# lokl:myproject - START
127.0.0.1 app.example.com
::1 app.example.com
127.0.0.1 api.example.com
::1 api.example.com
# lokl:myproject - END`

	if got != want {
		t.Errorf("block():\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestHostsManagerBlockEmpty(t *testing.T) {
	h := newDNSManager("myproject", nil, 0)

	got := h.block(nil)
	want := "# lokl:myproject - START\n# lokl:myproject - END"

	if got != want {
		t.Errorf("block(nil):\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddEmitsBothAddressFamilies(t *testing.T) {
	tmp := t.TempDir()
	hostsPath := filepath.Join(tmp, "hosts")
	if err := os.WriteFile(hostsPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	d := &dnsManager{project: "demo", hostsPath: hostsPath}
	if err := d.add([]string{"a.test", "b.test"}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(hostsPath)
	s := string(content)

	for _, host := range []string{"a.test", "b.test"} {
		if !strings.Contains(s, "127.0.0.1 "+host) {
			t.Errorf("missing IPv4 entry for %s in:\n%s", host, s)
		}
		if !strings.Contains(s, "::1 "+host) {
			t.Errorf("missing IPv6 entry for %s in:\n%s", host, s)
		}
	}
}

func TestBlockIncludesBothAddressFamilies(t *testing.T) {
	d := &dnsManager{project: "demo"}
	out := d.block([]string{"a.test"})
	if !strings.Contains(out, "127.0.0.1 a.test") {
		t.Errorf("block missing IPv4 line:\n%s", out)
	}
	if !strings.Contains(out, "::1 a.test") {
		t.Errorf("block missing IPv6 line:\n%s", out)
	}
}

func TestHostsManagerMarkers(t *testing.T) {
	h := newDNSManager("testproject", nil, 0)

	if h.startMarker() != "# lokl:testproject - START" {
		t.Errorf("startMarker() = %q", h.startMarker())
	}
	if h.endMarker() != "# lokl:testproject - END" {
		t.Errorf("endMarker() = %q", h.endMarker())
	}
}
