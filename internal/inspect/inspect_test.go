package inspect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferPort(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   int
	}{
		{"explicit --port flag", "vite --port 4000", 4000},
		{"explicit --port= flag", "vite --port=4000", 4000},
		{"PORT env var", "PORT=9090 node server.js", 9090},
		{"dash p flag", "next dev -p 3001", 3001},
		{"vite default", "vite dev", 5173},
		{"next default", "next dev", 3000},
		{"nuxt default", "nuxt dev", 3000},
		{"astro default", "astro dev", 4321},
		{"angular default", "angular serve", 4200},
		{"ng serve no match", "ng serve", 0},
		{"gatsby default", "gatsby develop", 8000},
		{"storybook default", "storybook dev", 6006},
		{"case insensitive", "VITE dev", 5173},
		{"explicit overrides default", "vite --port 9000", 9000},
		{"unknown script", "node custom-server.js", 0},
		{"empty script", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferPort(tt.script)
			if got != tt.want {
				t.Errorf("InferPort(%q) = %d, want %d", tt.script, got, tt.want)
			}
		})
	}
}

func TestSortScriptsByPriority(t *testing.T) {
	tests := []struct {
		name    string
		scripts map[string]string
		want    []string
	}{
		{
			"dev first",
			map[string]string{"build": "tsc", "dev": "vite", "start": "node ."},
			[]string{"dev", "start", "build"},
		},
		{
			"all priority scripts",
			map[string]string{"serve": "s", "dev": "d", "watch": "w", "develop": "dv", "start": "st"},
			[]string{"dev", "develop", "start", "serve", "watch"},
		},
		{
			"unknown alphabetical",
			map[string]string{"zebra": "z", "alpha": "a"},
			[]string{"alpha", "zebra"},
		},
		{
			"empty",
			map[string]string{},
			[]string{},
		},
		{
			"single",
			map[string]string{"test": "jest"},
			[]string{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SortScriptsByPriority(tt.scripts)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d = %q, want %q (full: %v)", i, got[i], tt.want[i], got)
					break
				}
			}
		})
	}
}

func TestInspect(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name":"my-app","scripts":{"dev":"vite"}}`)

	result, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	if result.ProjectName != filepath.Base(dir) {
		t.Errorf("ProjectName = %q, want %q", result.ProjectName, filepath.Base(dir))
	}
	if len(result.Services) != 1 {
		t.Fatalf("Services len = %d, want 1", len(result.Services))
	}
	if result.Services[0].Name != "my-app" {
		t.Errorf("Name = %q, want my-app", result.Services[0].Name)
	}
}

func TestInspectMonorepo(t *testing.T) {
	root := t.TempDir()

	// Root package.json (no scripts = skipped)
	writePackageJSON(t, root, `{"name":"monorepo"}`)

	// Two sub-packages
	webDir := filepath.Join(root, "packages", "web")
	apiDir := filepath.Join(root, "packages", "api")
	mkdirAll(t, webDir)
	mkdirAll(t, apiDir)
	writePackageJSON(t, webDir, `{"name":"@org/web","scripts":{"dev":"vite"}}`)
	writePackageJSON(t, apiDir, `{"name":"api","scripts":{"start":"node ."}}`)

	result, err := Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(result.Services) != 2 {
		t.Fatalf("Services len = %d, want 2", len(result.Services))
	}
}

func TestInspectSkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	writePackageJSON(t, root, `{"name":"app","scripts":{"dev":"vite"}}`)

	nmDir := filepath.Join(root, "node_modules", "some-dep")
	mkdirAll(t, nmDir)
	writePackageJSON(t, nmDir, `{"name":"some-dep","scripts":{"build":"tsc"}}`)

	result, err := Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(result.Services) != 1 {
		t.Errorf("Services len = %d, want 1 (node_modules should be skipped)", len(result.Services))
	}
}

func TestInspectScopedName(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name":"@myorg/cool-pkg","scripts":{"dev":"vite"}}`)

	result, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if result.Services[0].Name != "cool-pkg" {
		t.Errorf("Name = %q, want cool-pkg (scope stripped)", result.Services[0].Name)
	}
}

func TestInspectNoScripts(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name":"lib-only"}`)

	result, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(result.Services) != 0 {
		t.Errorf("Services len = %d, want 0 (no scripts)", len(result.Services))
	}
}

func TestDetectPackageManager(t *testing.T) {
	tests := []struct {
		name     string
		lockfile string
		want     string
	}{
		{"pnpm", "pnpm-lock.yaml", "pnpm"},
		{"yarn", "yarn.lock", "yarn"},
		{"bun", "bun.lockb", "bun"},
		{"npm", "package-lock.json", "npm"},
		{"default npm", "", "npm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.lockfile != "" {
				writeFile(t, filepath.Join(dir, tt.lockfile), "")
			}
			got := detectPackageManager(dir)
			if got != tt.want {
				t.Errorf("detectPackageManager() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectPackageManagerPriority(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "")
	writeFile(t, filepath.Join(dir, "package-lock.json"), "")

	got := detectPackageManager(dir)
	if got != "pnpm" {
		t.Errorf("detectPackageManager() = %q, want pnpm (highest priority)", got)
	}
}

func TestInspectSuggestedDomain(t *testing.T) {
	dir := t.TempDir()
	result, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	want := filepath.Base(dir) + ".dev"
	if result.SuggestedDomain != want {
		t.Errorf("SuggestedDomain = %q, want %q", result.SuggestedDomain, want)
	}
}

// --- Helpers ---

func writePackageJSON(t *testing.T, dir, content string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "package.json"), content)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
