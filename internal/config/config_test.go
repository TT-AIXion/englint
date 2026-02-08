package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Severity != SeverityError {
		t.Fatalf("expected default severity %q", SeverityError)
	}
	if len(cfg.Include) == 0 || len(cfg.Exclude) == 0 {
		t.Fatalf("expected default include/exclude")
	}
	if got := cfg.Allow; !reflect.DeepEqual(got, []string{"©", "→"}) {
		t.Fatalf("unexpected allow list: %v", got)
	}
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   Config
		want Config
	}{
		{
			name: "all defaults",
			in:   Config{},
			want: DefaultConfig(),
		},
		{
			name: "keep custom and normalize severity",
			in: Config{
				Include:  []string{"**/*.go"},
				Exclude:  []string{"vendor/**"},
				Allow:    []string{"§"},
				Severity: " WARNING ",
			},
			want: Config{
				Include:  []string{"**/*.go"},
				Exclude:  []string{"vendor/**"},
				Allow:    []string{"§"},
				Severity: SeverityWarning,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyDefaults(tt.in)
			if !reflect.DeepEqual(got.Include, tt.want.Include) {
				t.Fatalf("include mismatch: got %v want %v", got.Include, tt.want.Include)
			}
			if !reflect.DeepEqual(got.Exclude, tt.want.Exclude) {
				t.Fatalf("exclude mismatch: got %v want %v", got.Exclude, tt.want.Exclude)
			}
			if !reflect.DeepEqual(got.Allow, tt.want.Allow) {
				t.Fatalf("allow mismatch: got %v want %v", got.Allow, tt.want.Allow)
			}
			if got.Severity != tt.want.Severity {
				t.Fatalf("severity mismatch: got %q want %q", got.Severity, tt.want.Severity)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "valid", cfg: DefaultConfig(), wantErr: false},
		{name: "invalid severity", cfg: Config{Severity: "critical"}, wantErr: true},
		{name: "empty allow entry", cfg: Config{Severity: SeverityError, Allow: []string{""}}, wantErr: true},
		{name: "invalid utf8", cfg: Config{Severity: SeverityError, Allow: []string{string([]byte{0xff})}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("missing file uses defaults", func(t *testing.T) {
		cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if cfg.Severity != SeverityError {
			t.Fatalf("expected default severity")
		}
	})

	t.Run("valid file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".englint.yaml")
		content := `include:
  - "**/*.go"
exclude:
  - "vendor/**"
allow:
  - "©" # allowed
severity: warning
ignore_comments: true
ignore_strings: true
allow_file_patterns:
  - "docs/**"
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if cfg.Severity != SeverityWarning {
			t.Fatalf("expected warning severity")
		}
		if !cfg.IgnoreComments || !cfg.IgnoreStrings {
			t.Fatalf("expected ignore flags")
		}
		if len(cfg.AllowFilePatterns) != 1 {
			t.Fatalf("expected allow_file_patterns")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".englint.yaml")
		if err := os.WriteFile(path, []byte("include: [\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("expected load error")
		}
	})

	t.Run("invalid severity in file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".englint.yaml")
		if err := os.WriteFile(path, []byte("severity: bad\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("expected validation error")
		}
	})

	t.Run("read error", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := Load(dir); err == nil {
			t.Fatalf("expected read error")
		}
	})

	t.Run("parse error path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".englint.yaml")
		if err := os.WriteFile(path, []byte("severity: error\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		orig := parseYAML
		parseYAML = func(string) (Config, error) { return Config{}, errors.New("boom") }
		defer func() { parseYAML = orig }()
		if _, err := Load(path); err == nil {
			t.Fatalf("expected parse error")
		}
	})
}

func TestSave(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".englint.yaml")
		cfg := Config{
			Include:  []string{"**/*.go"},
			Exclude:  []string{"vendor/**"},
			Allow:    []string{"©"},
			Severity: SeverityWarning,
		}
		if err := Save(path, cfg); err != nil {
			t.Fatalf("Save returned error: %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if loaded.Severity != SeverityWarning {
			t.Fatalf("expected warning severity")
		}
		if len(loaded.Allow) != 1 || loaded.Allow[0] != "©" {
			t.Fatalf("unexpected allow list: %v", loaded.Allow)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".englint.yaml")
		if err := Save(path, Config{Severity: "bad"}); err == nil {
			t.Fatalf("expected validation error")
		}
	})

	t.Run("mkdir error", func(t *testing.T) {
		tmp := t.TempDir()
		parent := filepath.Join(tmp, "file-parent")
		if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
			t.Fatalf("write parent file: %v", err)
		}
		if err := Save(filepath.Join(parent, "config.yaml"), DefaultConfig()); err == nil {
			t.Fatalf("expected mkdir error")
		}
	})

	t.Run("render error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".englint.yaml")
		orig := renderYAML
		renderYAML = func(Config) (string, error) { return "", errors.New("render") }
		defer func() { renderYAML = orig }()
		if err := Save(path, DefaultConfig()); err == nil {
			t.Fatalf("expected render error")
		}
	})

	t.Run("write error", func(t *testing.T) {
		tmp := t.TempDir()
		readonly := filepath.Join(tmp, "readonly")
		if err := os.MkdirAll(readonly, 0o555); err != nil {
			t.Fatalf("mkdir readonly: %v", err)
		}
		defer os.Chmod(readonly, 0o755)
		if err := Save(filepath.Join(readonly, "config.yaml"), DefaultConfig()); err == nil {
			t.Fatalf("expected write error")
		}
	})
}

func TestWriteDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".englint.yaml")
	if err := WriteDefault(path); err != nil {
		t.Fatalf("WriteDefault returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if !strings.Contains(string(data), "severity: error") {
		t.Fatalf("default template missing severity")
	}

	tmp := t.TempDir()
	parentFile := filepath.Join(tmp, "parent")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	if err := WriteDefault(filepath.Join(parentFile, ".englint.yaml")); err == nil {
		t.Fatalf("expected write default error")
	}
}

func TestAllowedRuneMap(t *testing.T) {
	allow := []string{"©", "→", "ab"}
	m := AllowedRuneMap(allow)
	for _, r := range []rune{'©', '→', 'a', 'b'} {
		if _, ok := m[r]; !ok {
			t.Fatalf("missing rune %q", r)
		}
	}
}

func TestParseConfigYAMLAndHelpers(t *testing.T) {
	t.Run("parse scalar variants", func(t *testing.T) {
		cases := []struct {
			in      string
			want    string
			wantErr bool
		}{
			{in: "hello", want: "hello"},
			{in: "\"hello\"", want: "hello"},
			{in: "'hello'", want: "hello"},
			{in: "\"a#b\" # c", want: "a#b"},
			{in: "", wantErr: true},
			{in: "\"unterminated", wantErr: true},
			{in: "'unterminated", wantErr: true},
		}
		for _, tc := range cases {
			got, err := parseScalar(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				continue
			}
			if err != nil {
				t.Fatalf("parseScalar(%q) error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("parseScalar(%q) = %q, want %q", tc.in, got, tc.want)
			}
		}
	})

	t.Run("strip inline comment", func(t *testing.T) {
		if got := stripInlineComment("value # comment"); got != "value" {
			t.Fatalf("unexpected strip result: %q", got)
		}
		if got := stripInlineComment("\"a#b\" # comment"); got != "\"a#b\"" {
			t.Fatalf("unexpected strip result: %q", got)
		}
		if got := stripInlineComment("'a#b' # comment"); got != "'a#b'" {
			t.Fatalf("unexpected strip result: %q", got)
		}
		if got := stripInlineComment("\"a\\\"#b\" # comment"); got != "\"a\\\"#b\"" {
			t.Fatalf("unexpected strip result: %q", got)
		}
	})

	t.Run("yaml parse errors", func(t *testing.T) {
		cases := []string{
			"- orphan",
			"include: one",
			"unknown: true",
			"ignore_comments: maybe",
			"severity error",
		}
		for _, tc := range cases {
			if _, err := parseConfigYAML(tc); err == nil {
				t.Fatalf("expected parse error for %q", tc)
			}
		}
	})

	t.Run("render yaml", func(t *testing.T) {
		cfg := Config{
			Include:           []string{"**/*.go"},
			Exclude:           []string{"vendor/**"},
			Allow:             []string{"©"},
			Severity:          SeverityError,
			IgnoreComments:    true,
			IgnoreStrings:     true,
			AllowFilePatterns: []string{"docs/**"},
		}
		rendered, err := renderConfigYAML(cfg)
		if err != nil {
			t.Fatalf("renderConfigYAML error: %v", err)
		}
		for _, mustContain := range []string{"include:", "exclude:", "allow:", "severity: error", "ignore_comments: true", "allow_file_patterns:"} {
			if !strings.Contains(rendered, mustContain) {
				t.Fatalf("expected rendered YAML to contain %q", mustContain)
			}
		}
	})
}
