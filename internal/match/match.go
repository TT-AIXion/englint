package match

import (
	"path/filepath"
	"regexp"
	"strings"
)

var compileRegexp = regexp.Compile

// Any returns true when any glob pattern matches value.
func Any(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		if Match(pattern, value) {
			return true
		}
	}
	return false
}

// Match checks path-style glob matching with support for ** and ?.
func Match(pattern, value string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	value = filepath.ToSlash(strings.TrimSpace(value))
	if pattern == value {
		return true
	}
	if strings.HasPrefix(pattern, "**/") && !strings.Contains(value, "/") {
		if Match(strings.TrimPrefix(pattern, "**/"), value) {
			return true
		}
	}
	re, err := globToRegexp(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteString("$")
	return compileRegexp(b.String())
}
