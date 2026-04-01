package update

import (
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"v0.2.2", "v0.3.0", true},
		{"v0.2.2", "v0.2.2", false},
		{"v0.2.10", "v0.2.9", false},
		{"v0.2.9", "v0.2.10", true},
		{"v0.9.9", "v1.0.0", true},
		{"v1.0.0", "v0.9.9", false},
		{"v1.0.0", "v1.0.0", false},
	}
	for _, tc := range tests {
		got := isNewer(tc.current, tc.latest)
		if got != tc.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestShouldSkip_DevBuild(t *testing.T) {
	if !shouldSkip("dev") {
		t.Error("expected skip for dev build")
	}
}

func TestShouldSkip_CI(t *testing.T) {
	t.Setenv("CI", "true")
	if !shouldSkip("v0.2.0") {
		t.Error("expected skip when CI is set")
	}
}

func TestShouldSkip_OptOut(t *testing.T) {
	t.Setenv("LOKL_NO_UPDATE_CHECK", "1")
	if !shouldSkip("v0.2.0") {
		t.Error("expected skip when LOKL_NO_UPDATE_CHECK is set")
	}
}

func TestShouldSkip_Normal(t *testing.T) {
	// Ensure env vars are unset.
	t.Setenv("CI", "")
	t.Setenv("LOKL_NO_UPDATE_CHECK", "")
	if shouldSkip("v0.2.0") {
		t.Error("expected no skip for normal build")
	}
}

func TestReadWriteCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	c := &cache{
		LatestVersion: "v0.5.0",
		CheckedAt:     time.Now().UTC().Truncate(time.Second),
	}
	if err := writeCache(c); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	got, err := readCache()
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if got.LatestVersion != c.LatestVersion {
		t.Errorf("LatestVersion: got %q, want %q", got.LatestVersion, c.LatestVersion)
	}
	if !got.CheckedAt.Equal(c.CheckedAt) {
		t.Errorf("CheckedAt: got %v, want %v", got.CheckedAt, c.CheckedAt)
	}
}

func TestCacheStale(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	c := &cache{
		LatestVersion: "v0.5.0",
		CheckedAt:     time.Now().UTC().Add(-25 * time.Hour),
	}
	if err := writeCache(c); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	got, err := readCache()
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if time.Since(got.CheckedAt) < cacheTTL {
		t.Error("expected cache to be stale (> 24h old)")
	}
}

func TestCacheFresh(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	c := &cache{
		LatestVersion: "v0.5.0",
		CheckedAt:     time.Now().UTC().Add(-1 * time.Hour),
	}
	if err := writeCache(c); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	got, err := readCache()
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if time.Since(got.CheckedAt) >= cacheTTL {
		t.Error("expected cache to be fresh (< 24h old)")
	}
}
