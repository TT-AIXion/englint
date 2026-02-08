package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failure")
}

func TestParseScanArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, got scanArgs)
	}{
		{
			name: "defaults",
			args: nil,
			check: func(t *testing.T, got scanArgs) {
				if got.ConfigPath != ".englint.yaml" {
					t.Fatalf("unexpected config path: %q", got.ConfigPath)
				}
				if len(got.Paths) != 1 || got.Paths[0] != "." {
					t.Fatalf("unexpected default paths: %v", got.Paths)
				}
			},
		},
		{
			name: "flags and paths",
			args: []string{"src", "--json", "--config", "cfg.yaml", "--exclude", "vendor/**", "--include=**/*.go", "--severity", "warning", "--fix", "--no-color", "--verbose"},
			check: func(t *testing.T, got scanArgs) {
				if !got.JSON || !got.Fix || !got.NoColor || !got.Verbose {
					t.Fatalf("expected bool flags true: %+v", got)
				}
				if got.ConfigPath != "cfg.yaml" {
					t.Fatalf("unexpected config path: %q", got.ConfigPath)
				}
				if len(got.Exclude) != 1 || got.Exclude[0] != "vendor/**" {
					t.Fatalf("unexpected exclude: %v", got.Exclude)
				}
				if len(got.Include) != 1 || got.Include[0] != "**/*.go" {
					t.Fatalf("unexpected include: %v", got.Include)
				}
				if got.Severity != "warning" {
					t.Fatalf("unexpected severity: %q", got.Severity)
				}
			},
		},
		{
			name: "equals variants",
			args: []string{"--config=", "--exclude=vendor/**", "--include", "**/*.md", "--severity=ERROR"},
			check: func(t *testing.T, got scanArgs) {
				if got.ConfigPath != ".englint.yaml" {
					t.Fatalf("expected empty config path to fall back to default, got %q", got.ConfigPath)
				}
				if got.Severity != "error" {
					t.Fatalf("expected lowercased severity, got %q", got.Severity)
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--bad"},
			wantErr: true,
		},
		{
			name:    "missing value",
			args:    []string{"--config"},
			wantErr: true,
		},
		{
			name:    "missing include value",
			args:    []string{"--include"},
			wantErr: true,
		},
		{
			name:    "missing exclude value",
			args:    []string{"--exclude"},
			wantErr: true,
		},
		{
			name:    "missing severity value",
			args:    []string{"--severity"},
			wantErr: true,
		},
		{
			name: "double dash",
			args: []string{"--config=abc", "--", "--not-flag", "path"},
			check: func(t *testing.T, got scanArgs) {
				if len(got.Paths) != 2 || got.Paths[0] != "--not-flag" {
					t.Fatalf("unexpected paths: %v", got.Paths)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScanArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseScanArgs error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestParseInitArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		want    string
	}{
		{name: "default", args: nil, want: ".englint.yaml"},
		{name: "with equals", args: []string{"--config=cfg.yaml"}, want: "cfg.yaml"},
		{name: "with value", args: []string{"--config", "cfg.yaml"}, want: "cfg.yaml"},
		{name: "empty config", args: []string{"--config="}, want: ".englint.yaml"},
		{name: "ignore empty arg", args: []string{""}, want: ".englint.yaml"},
		{name: "unknown", args: []string{"--bad"}, wantErr: true},
		{name: "missing", args: []string{"--config"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInitArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInitArgs error: %v", err)
			}
			if got.ConfigPath != tt.want {
				t.Fatalf("unexpected config path: %q", got.ConfigPath)
			}
		})
	}
}

func TestRunMainVersionAndUsage(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer
	if code := runMain([]string{"version"}, &out, &errBuf); code != 0 {
		t.Fatalf("expected 0")
	}
	if !strings.Contains(out.String(), "englint") {
		t.Fatalf("expected version output")
	}

	out.Reset()
	if code := runMain(nil, &out, &errBuf); code != 0 {
		t.Fatalf("expected 0")
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("expected usage output")
	}

	errBuf.Reset()
	if code := runMain([]string{"unknown"}, &out, &errBuf); code != 1 {
		t.Fatalf("expected 1 for unknown command")
	}
	if !strings.Contains(errBuf.String(), "unknown command") {
		t.Fatalf("expected unknown command error")
	}

	out.Reset()
	if code := runMain([]string{"help"}, &out, &errBuf); code != 0 {
		t.Fatalf("expected help success")
	}
	if !strings.Contains(out.String(), "englint - detect non-English text") {
		t.Fatalf("expected help text")
	}
}

func TestRunInit(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, ".englint.yaml")
	var out bytes.Buffer
	var errBuf bytes.Buffer

	if code := runMain([]string{"init", "--config", configPath}, &out, &errBuf); code != 0 {
		t.Fatalf("expected init success, got %d, err=%s", code, errBuf.String())
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
	if !strings.Contains(out.String(), "Created") {
		t.Fatalf("expected success message")
	}

	errBuf.Reset()
	if code := runMain([]string{"init", "--config", configPath}, &out, &errBuf); code != 1 {
		t.Fatalf("expected init failure for existing file")
	}
	if !strings.Contains(errBuf.String(), "already exists") {
		t.Fatalf("expected existing file error")
	}
}

func TestRunInitErrors(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	if code := runMain([]string{"init", "--bad"}, &out, &errBuf); code != 1 {
		t.Fatalf("expected init argument failure")
	}
	if !strings.Contains(errBuf.String(), "init argument error") {
		t.Fatalf("expected init argument error")
	}

	tmp := t.TempDir()
	parentFile := filepath.Join(tmp, "parent-file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	errBuf.Reset()
	if code := runMain([]string{"init", "--config", filepath.Join(parentFile, ".englint.yaml")}, &out, &errBuf); code != 1 {
		t.Fatalf("expected write failure")
	}
	if !strings.Contains(errBuf.String(), "failed to create config") && !strings.Contains(errBuf.String(), "failed to check config file") {
		t.Fatalf("expected create config error, got %s", errBuf.String())
	}

	blockedDir := filepath.Join(tmp, "blocked")
	if err := os.MkdirAll(blockedDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(blockedDir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(blockedDir, 0o700)
	errBuf.Reset()
	code := runMain([]string{"init", "--config", filepath.Join(blockedDir, ".englint.yaml")}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("expected stat/check failure")
	}
	if !strings.Contains(errBuf.String(), "failed to check config file") && !strings.Contains(errBuf.String(), "failed to create config") {
		t.Fatalf("expected check/create failure message, got %s", errBuf.String())
	}
}

func TestRunScan(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, ".englint.yaml")
	sourcePath := filepath.Join(tmp, "sample.go")
	asciiPath := filepath.Join(tmp, "ascii.go")

	cfg := `include:
  - "**/*.go"
exclude:
  - "vendor/**"
allow:
  - "©"
severity: error
`
	if err := os.WriteFile(configPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("package p\nvar _ = \"こんにちは\"\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(asciiPath, []byte("package p\nvar _ = \"hello\"\n"), 0o644); err != nil {
		t.Fatalf("write ascii source: %v", err)
	}

	var out bytes.Buffer
	var errBuf bytes.Buffer
	if code := runMain([]string{"scan", "--config", configPath, sourcePath, "--no-color", "--fix", "--verbose"}, &out, &errBuf); code != 1 {
		t.Fatalf("expected scan with findings to return 1, got %d, err=%s", code, errBuf.String())
	}
	text := out.String()
	for _, expected := range []string{"ERROR", "Summary:", "Auto-fix is not implemented yet.", "SCANNED"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected output to contain %q\nactual:\n%s", expected, text)
		}
	}

	out.Reset()
	errBuf.Reset()
	if code := runMain([]string{"scan", "--config", configPath, asciiPath, "--json", "--severity", "warning"}, &out, &errBuf); code != 0 {
		t.Fatalf("expected scan without findings to return 0, got %d, err=%s", code, errBuf.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json output: %v", err)
	}
	if payload["summary"] == nil {
		t.Fatalf("expected summary in json output")
	}

	out.Reset()
	errBuf.Reset()
	if code := runMain([]string{"scan", "--config", configPath, sourcePath, "--exclude", "**/*.go", "--no-color"}, &out, &errBuf); code != 0 {
		t.Fatalf("expected exclusion to produce no findings, got %d", code)
	}
	if !strings.Contains(out.String(), "No non-English text found") {
		t.Fatalf("expected clean scan output")
	}
}

func TestRunScanErrors(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "bad.yaml")
	if err := os.WriteFile(configPath, []byte("severity: invalid\n"), 0o644); err != nil {
		t.Fatalf("write bad config: %v", err)
	}
	var out bytes.Buffer
	var errBuf bytes.Buffer

	if code := runMain([]string{"scan", "--config", configPath, tmp}, &out, &errBuf); code != 1 {
		t.Fatalf("expected validation error code")
	}
	if !strings.Contains(errBuf.String(), "config error") {
		t.Fatalf("expected validation message, got: %s", errBuf.String())
	}

	errBuf.Reset()
	if code := runMain([]string{"scan", "--bad"}, &out, &errBuf); code != 1 {
		t.Fatalf("expected argument error")
	}
	if !strings.Contains(errBuf.String(), "scan argument error") {
		t.Fatalf("expected argument error message")
	}

	errBuf.Reset()
	if code := runMain([]string{"scan", "--config", tmp, tmp}, &out, &errBuf); code != 1 {
		t.Fatalf("expected config load/read error")
	}
	if !strings.Contains(errBuf.String(), "config error") {
		t.Fatalf("expected config load error")
	}

	errBuf.Reset()
	if code := runMain([]string{"scan", "--config", filepath.Join(tmp, "missing.yaml"), filepath.Join(tmp, "does-not-exist")}, &out, &errBuf); code != 1 {
		t.Fatalf("expected scan failure")
	}
	if !strings.Contains(errBuf.String(), "scan error") {
		t.Fatalf("expected scan error")
	}
}

func TestRunScanOutputError(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "ok.go")
	if err := os.WriteFile(filePath, []byte("package p\nvar _ = \"hello\"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	var errBuf bytes.Buffer
	if code := runMain([]string{"scan", filePath}, failWriter{}, &errBuf); code != 1 {
		t.Fatalf("expected output error code")
	}
	if !strings.Contains(errBuf.String(), "output error") {
		t.Fatalf("expected output error message")
	}
}

func TestMainFunction(t *testing.T) {
	origExit := exitFunc
	origArgs := os.Args
	defer func() {
		exitFunc = origExit
		os.Args = origArgs
	}()

	called := false
	exitFunc = func(int) { called = true }
	os.Args = []string{"englint", "version"}
	main()
	if !called {
		t.Fatalf("expected exit function to be called")
	}
}
