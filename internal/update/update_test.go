package update

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
		{"v1.2", "v1.2.0", false}, // trailing zero not newer
		{"v1.2.0", "v1.2", false}, // shorter not newer
		{"v1.2", "v1.2.1", true},  // trailing non-zero is newer
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

func sandboxCache(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", tmp) // Linux: os.UserCacheDir() checks this first
}

func TestReadWriteCache(t *testing.T) {
	sandboxCache(t)

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
	sandboxCache(t)

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
	sandboxCache(t)

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

func TestLatestVersionCacheHit(t *testing.T) {
	sandboxCache(t)

	// Write a fresh cache — latestVersion should return it without any HTTP call.
	_ = writeCache(&cache{LatestVersion: "v0.9.0", CheckedAt: time.Now().UTC()})

	origURL := apiURL
	apiURL = "http://127.0.0.1:0/should-not-be-called"
	defer func() { apiURL = origURL }()

	if got := latestVersion(); got != "v0.9.0" {
		t.Errorf("latestVersion() = %q, want cache hit %q", got, "v0.9.0")
	}
}

func TestLatestVersionFetch(t *testing.T) {
	sandboxCache(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v0.9.0"}`)
	}))
	defer ts.Close()

	origURL := apiURL
	apiURL = ts.URL
	defer func() { apiURL = origURL }()

	if got := latestVersion(); got != "v0.9.0" {
		t.Errorf("latestVersion() = %q, want %q", got, "v0.9.0")
	}

	// Cache should be written.
	c, err := readCache()
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if c.LatestVersion != "v0.9.0" {
		t.Errorf("cached version = %q, want %q", c.LatestVersion, "v0.9.0")
	}
}

func TestLatestVersionFetchFailureCached(t *testing.T) {
	sandboxCache(t)

	// Server that always fails.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	origURL := apiURL
	apiURL = ts.URL
	defer func() { apiURL = origURL }()

	start := time.Now()
	got := latestVersion()
	if got != "" {
		t.Errorf("expected empty on fetch failure, got %q", got)
	}

	// Failure should be cached so next call skips HTTP entirely.
	c, err := readCache()
	if err != nil {
		t.Fatalf("readCache after failure: %v", err)
	}
	if c.LatestVersion != "" {
		t.Errorf("expected empty cached version after failure, got %q", c.LatestVersion)
	}
	if c.CheckedAt.Before(start) {
		t.Errorf("expected CheckedAt >= %v, got %v", start, c.CheckedAt)
	}
}
