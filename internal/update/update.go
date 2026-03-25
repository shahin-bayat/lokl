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
	apiURL      = "https://api.github.com/repos/shahin-bayat/lokl/releases/latest"
	httpTimeout = 5 * time.Second
)

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

	latest, err := fetchLatest()
	if err != nil {
		return ""
	}

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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func cachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lokl", "version-check.json"), nil
}

func fetchLatest() (string, error) {
	resp, err := httpClient.Get(apiURL)
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
// Both strings may have a leading "v".
func isNewer(current, latest string) bool {
	cv := parseVersion(current)
	lv := parseVersion(latest)
	for i := range cv {
		if i >= len(lv) {
			return false
		}
		if lv[i] > cv[i] {
			return true
		}
		if lv[i] < cv[i] {
			return false
		}
	}
	return len(lv) > len(cv)
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
