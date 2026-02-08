package match

import (
	"errors"
	"regexp"
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "prefix wildcard", pattern: "foo*", value: "foobar", want: true},
		{name: "single wildcard", pattern: "foo?", value: "fooa", want: true},
		{name: "single wildcard miss", pattern: "foo?", value: "foo", want: false},
		{name: "double star nested", pattern: "**/bar", value: "a/b/bar", want: true},
		{name: "double star root", pattern: "**/bar", value: "bar", want: true},
		{name: "exact", pattern: "a/b/c.go", value: "a/b/c.go", want: true},
		{name: "exact miss", pattern: "a/b/c.go", value: "a/b/d.go", want: false},
		{name: "special chars", pattern: "a+b/*.go", value: "a+b/x.go", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.pattern, tt.value)
			if got != tt.want {
				t.Fatalf("Match(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestAny(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		value    string
		want     bool
	}{
		{name: "one matches", patterns: []string{"", "*.go", "*.md"}, value: "main.go", want: true},
		{name: "none matches", patterns: []string{"*.ts", "*.tsx"}, value: "main.go", want: false},
		{name: "blank patterns ignored", patterns: []string{" ", "**/*.go"}, value: "dir/a.go", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Any(tt.patterns, tt.value); got != tt.want {
				t.Fatalf("Any(%v, %q) = %v, want %v", tt.patterns, tt.value, got, tt.want)
			}
		})
	}
}

func TestMatchCompileError(t *testing.T) {
	orig := compileRegexp
	compileRegexp = func(string) (*regexp.Regexp, error) {
		return nil, errors.New("compile")
	}
	defer func() { compileRegexp = orig }()

	if Match("a*", "abc") {
		t.Fatalf("expected false on compile error")
	}
	if !Match("abc", "abc") {
		t.Fatalf("exact match should still work before regexp")
	}
}
