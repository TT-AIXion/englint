package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

const DefaultTemplate = `include:
  - "**/*.ts"
  - "**/*.tsx"
  - "**/*.go"
  - "**/*.md"
exclude:
  - "node_modules/**"
  - ".git/**"
  - "vendor/**"
  - "*.lock"
allow:
  - "©"  # copyright symbol
  - "→"  # arrow
severity: error
# ignore_comments: false
# ignore_strings: false
# allow_file_patterns:
#   - "docs/**"
`

type Config struct {
	Include           []string
	Exclude           []string
	Allow             []string
	Severity          string
	IgnoreComments    bool
	IgnoreStrings     bool
	AllowFilePatterns []string
}

var parseYAML = parseConfigYAML
var renderYAML = renderConfigYAML

func DefaultConfig() Config {
	return Config{
		Include:           []string{"**/*.ts", "**/*.tsx", "**/*.go", "**/*.md"},
		Exclude:           []string{"node_modules/**", ".git/**", "vendor/**", "*.lock"},
		Allow:             []string{"©", "→"},
		Severity:          SeverityError,
		IgnoreComments:    false,
		IgnoreStrings:     false,
		AllowFilePatterns: nil,
	}
}

func ApplyDefaults(cfg Config) Config {
	defaults := DefaultConfig()
	if len(cfg.Include) == 0 {
		cfg.Include = defaults.Include
	}
	if len(cfg.Exclude) == 0 {
		cfg.Exclude = defaults.Exclude
	}
	if cfg.Allow == nil {
		cfg.Allow = defaults.Allow
	}
	if strings.TrimSpace(cfg.Severity) == "" {
		cfg.Severity = defaults.Severity
	}
	cfg.Severity = strings.ToLower(strings.TrimSpace(cfg.Severity))
	return cfg
}

func Validate(cfg Config) error {
	if cfg.Severity != SeverityError && cfg.Severity != SeverityWarning {
		return fmt.Errorf("severity must be %q or %q", SeverityError, SeverityWarning)
	}
	for _, v := range cfg.Allow {
		if strings.TrimSpace(v) == "" {
			return errors.New("allow values must not be empty")
		}
		if !utf8.ValidString(v) {
			return errors.New("allow values must be valid UTF-8")
		}
	}
	return nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := ApplyDefaults(Config{})
			if err := Validate(cfg); err != nil {
				return Config{}, err
			}
			return cfg, nil
		}
		return Config{}, err
	}

	cfg, err := parseYAML(string(data))
	if err != nil {
		return Config{}, fmt.Errorf("invalid YAML in %s: %w", path, err)
	}
	cfg = ApplyDefaults(cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg = ApplyDefaults(cfg)
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := renderYAML(cfg)
	if err != nil {
		return err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data += "\n"
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

func WriteDefault(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(DefaultTemplate), 0o644)
}

func AllowedRuneMap(allow []string) map[rune]struct{} {
	out := make(map[rune]struct{})
	for _, item := range allow {
		for _, r := range item {
			out[r] = struct{}{}
		}
	}
	return out
}

func parseConfigYAML(input string) (Config, error) {
	cfg := Config{}
	currentList := ""
	lines := strings.Split(input, "\n")

	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			if currentList == "" {
				return Config{}, fmt.Errorf("line %d: list item without key", lineNo)
			}
			value, err := parseScalar(strings.TrimSpace(strings.TrimPrefix(line, "- ")))
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			switch currentList {
			case "include":
				cfg.Include = append(cfg.Include, value)
			case "exclude":
				cfg.Exclude = append(cfg.Exclude, value)
			case "allow":
				cfg.Allow = append(cfg.Allow, value)
			case "allow_file_patterns":
				cfg.AllowFilePatterns = append(cfg.AllowFilePatterns, value)
			default:
				return Config{}, fmt.Errorf("line %d: key %q does not support list values", lineNo, currentList)
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return Config{}, fmt.Errorf("line %d: expected key: value", lineNo)
		}
		key := strings.TrimSpace(parts[0])
		valueRaw := strings.TrimSpace(parts[1])
		currentList = ""
		if valueRaw == "" {
			currentList = key
			continue
		}

		value, err := parseScalar(valueRaw)
		if err != nil {
			return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
		}

		switch key {
		case "severity":
			cfg.Severity = value
		case "ignore_comments":
			cfg.IgnoreComments, err = strconv.ParseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: ignore_comments must be true or false", lineNo)
			}
		case "ignore_strings":
			cfg.IgnoreStrings, err = strconv.ParseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: ignore_strings must be true or false", lineNo)
			}
		case "include", "exclude", "allow", "allow_file_patterns":
			return Config{}, fmt.Errorf("line %d: key %q requires list values", lineNo, key)
		default:
			return Config{}, fmt.Errorf("line %d: unknown key %q", lineNo, key)
		}
	}

	return cfg, nil
}

func parseScalar(value string) (string, error) {
	value = strings.TrimSpace(stripInlineComment(value))
	if value == "" {
		return "", errors.New("empty value")
	}
	if strings.HasPrefix(value, "\"") {
		unq, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string %q", value)
		}
		return unq, nil
	}
	if strings.HasPrefix(value, "'") {
		if !strings.HasSuffix(value, "'") || len(value) < 2 {
			return "", fmt.Errorf("invalid single-quoted string %q", value)
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
		inner = strings.ReplaceAll(inner, "''", "'")
		return inner, nil
	}
	return value, nil
}

func stripInlineComment(line string) string {
	inSingle := false
	inDouble := false
	escaped := false

	for i, r := range line {
		switch r {
		case '\\':
			if inDouble {
				escaped = !escaped
			} else {
				escaped = false
			}
		case '"':
			if !inSingle && !escaped {
				inDouble = !inDouble
			}
			escaped = false
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			escaped = false
		case '#':
			if !inSingle && !inDouble {
				return strings.TrimSpace(line[:i])
			}
			escaped = false
		default:
			escaped = false
		}
	}
	return strings.TrimSpace(line)
}

func renderConfigYAML(cfg Config) (string, error) {
	var b strings.Builder
	writeList(&b, "include", cfg.Include)
	writeList(&b, "exclude", cfg.Exclude)
	writeList(&b, "allow", cfg.Allow)
	b.WriteString("severity: ")
	b.WriteString(cfg.Severity)
	b.WriteByte('\n')
	if cfg.IgnoreComments {
		b.WriteString("ignore_comments: true\n")
	}
	if cfg.IgnoreStrings {
		b.WriteString("ignore_strings: true\n")
	}
	if len(cfg.AllowFilePatterns) > 0 {
		writeList(&b, "allow_file_patterns", cfg.AllowFilePatterns)
	}
	return b.String(), nil
}

func writeList(b *strings.Builder, key string, values []string) {
	b.WriteString(key)
	b.WriteString(":\n")
	for _, value := range values {
		b.WriteString("  - ")
		b.WriteString(strconv.Quote(value))
		b.WriteByte('\n')
	}
}
