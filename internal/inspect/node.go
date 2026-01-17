package inspect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type nodeInspector struct{}

func (n *nodeInspector) name() string { return "node" }

func (n *nodeInspector) inspect(root string) ([]Service, error) {
	// Entry check: is this a node project?
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		return nil, nil
	}

	pm := detectPackageManager(root)
	var services []Service

	// Walk tree to find all package.json files
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip node_modules and hidden directories
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Name() != "package.json" {
			return nil
		}

		svc := parsePackageJSON(path, root, pm)
		if svc != nil {
			services = append(services, *svc)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return services, nil
}

func detectPackageManager(root string) string {
	lockfiles := []struct {
		file string
		pm   string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"package-lock.json", "npm"},
	}

	for _, lf := range lockfiles {
		if _, err := os.Stat(filepath.Join(root, lf.file)); err == nil {
			return lf.pm
		}
	}
	return "npm"
}

type packageJSON struct {
	Name    string            `json:"name"`
	Scripts map[string]string `json:"scripts"`
}

var portPattern = regexp.MustCompile(`(?:--port[=\s]|PORT=|-p\s?)(\d+)`)

var devScriptPriority = map[string]int{
	"dev":     1,
	"develop": 2,
	"start":   3,
	"serve":   4,
	"watch":   5,
}

// SortScriptsByPriority returns script names sorted with common dev scripts first.
func SortScriptsByPriority(scripts map[string]string) []string {
	names := make([]string, 0, len(scripts))
	for name := range scripts {
		names = append(names, name)
	}

	sort.Slice(names, func(i, j int) bool {
		pi, pj := devScriptPriority[names[i]], devScriptPriority[names[j]]
		if pi != pj {
			if pi == 0 {
				return false
			}
			if pj == 0 {
				return true
			}
			return pi < pj
		}
		return names[i] < names[j]
	})

	return names
}

var defaultPorts = map[string]int{
	"vite":      5173,
	"next":      3000,
	"nuxt":      3000,
	"remix":     5173,
	"astro":     4321,
	"svelte":    5173,
	"angular":   4200,
	"gatsby":    8000,
	"storybook": 6006,
}

// InferPort tries to extract port from a script command string.
// First checks for explicit port flags, then falls back to known tool defaults.
// Returns 0 if no port found.
func InferPort(script string) int {
	// Check explicit port flags first
	matches := portPattern.FindStringSubmatch(script)
	if len(matches) > 1 {
		if port, err := strconv.Atoi(matches[1]); err == nil {
			return port
		}
	}

	// Fall back to known tool defaults
	scriptLower := strings.ToLower(script)
	for tool, port := range defaultPorts {
		if strings.Contains(scriptLower, tool) {
			return port
		}
	}

	return 0
}

// GetScripts returns available npm scripts for a service path.
// Returns map of script name to script command.
func GetScripts(servicePath string) (map[string]string, error) {
	pkgPath := filepath.Join(servicePath, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, err
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	return pkg.Scripts, nil
}

func parsePackageJSON(path, root, pm string) *Service {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	// Must have at least one script to be runnable
	if len(pkg.Scripts) == 0 {
		return nil
	}

	dir := filepath.Dir(path)
	relPath, _ := filepath.Rel(root, dir)
	if relPath == "" {
		relPath = "."
	}

	name := pkg.Name
	if name == "" {
		name = filepath.Base(dir)
	}
	// Strip scope from name (@org/pkg -> pkg)
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	return &Service{
		Name:      name,
		Path:      relPath,
		Command:   pm, // just the package manager, init will append "run <script>"
		Port:      0,
		AutoStart: true,
	}
}
