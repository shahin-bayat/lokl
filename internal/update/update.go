package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	cacheTTL    = 24 * time.Hour
	httpTimeout = 5 * time.Second
)

// apiURL is a var so tests can point it at a local httptest server.
var apiURL = "https://api.github.com/repos/shahin-bayat/lokl/releases/latest"

var httpClient = &http.Client{Timeout: httpTimeout}

type cache struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

// Check returns the latest version string if an update is available,
// empty string otherwise. Silent on all errors.
func Check(current string) string {
	if shouldSkip(current) {
		return ""
	}

	latest := latestVersion()
	if latest == "" {
		return ""
	}

	if isNewer(current, latest) {
		return latest
	}
	return ""
}

func shouldSkip(current string) bool {
	if current == "dev" {
		return true
	}
	if os.Getenv("CI") != "" {
		return true
	}
	if os.Getenv("LOKL_NO_UPDATE_CHECK") != "" {
		return true
	}
	return false
}

func latestVersion() string {
	if c, err := readCache(); err == nil && time.Since(c.CheckedAt) < cacheTTL {
		return c.LatestVersion
	}

	latest, _ := fetchLatest()
	// Cache even on failure (empty LatestVersion) so transient errors and
	// rate-limit responses are throttled by cacheTTL, not retried every run.
	_ = writeCache(&cache{LatestVersion: latest, CheckedAt: time.Now().UTC()})
	return latest
}

func readCache() (*cache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func writeCache(c *cache) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	// Write atomically via temp file + rename to avoid corrupt reads.
	tmp, err := os.CreateTemp(dir, "version-check-*.json.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

func cachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lokl", "version-check.json"), nil
}

func fetchLatest() (string, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "lokl-cli")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.TagName, nil
}

// isNewer returns true if latest is strictly greater than current.
// Both strings may have a leading "v". Missing trailing segments are treated
// as 0, so v1.2 == v1.2.0 and v1.2.1 > v1.2.
func isNewer(current, latest string) bool {
	cv := parseVersion(current)
	lv := parseVersion(latest)
	n := len(cv)
	if len(lv) > n {
		n = len(lv)
	}
	for i := range n {
		c, l := 0, 0
		if i < len(cv) {
			c = cv[i]
		}
		if i < len(lv) {
			l = lv[i]
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
