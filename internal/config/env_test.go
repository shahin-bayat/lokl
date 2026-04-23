package config

import "testing"

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "valid file",
			path: "testdata/test.env",
			want: map[string]string{
				"DB_USER":    "postgres",
				"DB_PASS":    "s3cret",
				"CACHE_URL":  "redis://localhost:6379",
				"EMPTY_VAL":  "",
				"SPACED_KEY": "spaced_value",
			},
		},
		{
			name:    "file not found",
			path:    "testdata/nonexistent.env",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnvFile(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("%s = %q, want %q", k, got[k], want)
				}
			}
			if len(got) != len(tt.want) {
				t.Errorf("got %d keys, want %d", len(got), len(tt.want))
			}
		})
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{"hello", "hello"},
		{`"mismatched'`, `"mismatched'`},
		{"", ""},
		{`""`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := unquote(tt.in); got != tt.want {
				t.Errorf("unquote(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestResolveEnv(t *testing.T) {
	t.Run("interpolation from os env", func(t *testing.T) {
		t.Setenv("TEST_HOST", "myhost")

		cfg := &Config{
			Name: "test",
			Env:  map[string]string{"URL": "http://${TEST_HOST}:8080"},
			Services: map[string]Service{
				"api": {Command: cmdStr("x"), Env: map[string]string{"DSN": "postgres://$TEST_HOST/db"}},
			},
		}

		if err := resolveEnv(cfg, "."); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Env["URL"] != "http://myhost:8080" {
			t.Errorf("global URL = %q, want %q", cfg.Env["URL"], "http://myhost:8080")
		}
		if cfg.Services["api"].Env["DSN"] != "postgres://myhost/db" {
			t.Errorf("service DSN = %q, want %q", cfg.Services["api"].Env["DSN"], "postgres://myhost/db")
		}
	})

	t.Run("missing var expands to empty", func(t *testing.T) {
		cfg := &Config{
			Name:     "test",
			Env:      map[string]string{"VAL": "prefix-${SURELY_MISSING_VAR_XYZZY}-suffix"},
			Services: map[string]Service{},
		}

		if err := resolveEnv(cfg, "."); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Env["VAL"] != "prefix--suffix" {
			t.Errorf("VAL = %q, want %q", cfg.Env["VAL"], "prefix--suffix")
		}
	})

	t.Run("env_file loads values", func(t *testing.T) {
		cfg := &Config{
			Name:     "test",
			EnvFile:  []string{"testdata/test.env"},
			Services: map[string]Service{},
		}

		if err := resolveEnv(cfg, "."); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Env["DB_USER"] != "postgres" {
			t.Errorf("DB_USER = %q, want %q", cfg.Env["DB_USER"], "postgres")
		}
	})

	t.Run("inline env overrides env_file", func(t *testing.T) {
		cfg := &Config{
			Name:    "test",
			EnvFile: []string{"testdata/test.env"},
			Env:     map[string]string{"DB_USER": "admin"},
			Services: map[string]Service{
				"api": {
					Command: cmdStr("x"),
					EnvFile: []string{"testdata/test.env"},
					Env:     map[string]string{"DB_PASS": "override"},
				},
			},
		}

		if err := resolveEnv(cfg, "."); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Env["DB_USER"] != "admin" {
			t.Errorf("global DB_USER = %q, want %q", cfg.Env["DB_USER"], "admin")
		}
		if cfg.Services["api"].Env["DB_PASS"] != "override" {
			t.Errorf("service DB_PASS = %q, want %q", cfg.Services["api"].Env["DB_PASS"], "override")
		}
	})

	t.Run("interpolation resolves from env_file values", func(t *testing.T) {
		cfg := &Config{
			Name:    "test",
			EnvFile: []string{"testdata/test.env"},
			Services: map[string]Service{
				"api": {
					Command: cmdStr("x"),
					Env:     map[string]string{"DSN": "postgres://${DB_USER}:${DB_PASS}@localhost/db"},
				},
			},
		}

		if err := resolveEnv(cfg, "."); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "postgres://postgres:s3cret@localhost/db"
		if cfg.Services["api"].Env["DSN"] != want {
			t.Errorf("DSN = %q, want %q", cfg.Services["api"].Env["DSN"], want)
		}
	})

	t.Run("missing env_file returns error", func(t *testing.T) {
		cfg := &Config{
			Name:     "test",
			EnvFile:  []string{"nonexistent.env"},
			Services: map[string]Service{},
		}

		if err := resolveEnv(cfg, "."); err == nil {
			t.Fatal("expected error for missing env_file")
		}
	})
}
