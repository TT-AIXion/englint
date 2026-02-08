package scanner

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/TT-AIXion/englint/internal/match"
)

// Severity indicates result importance in output rendering.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Options controls scan behavior.
type Options struct {
	Include           []string
	Exclude           []string
	AllowRunes        map[rune]struct{}
	Severity          Severity
	IgnoreComments    bool
	IgnoreStrings     bool
	AllowFilePatterns []string
}

// Finding is a single non-English character detection.
type Finding struct {
	Path      string   `json:"path"`
	Line      int      `json:"line"`
	Column    int      `json:"column"`
	Character string   `json:"character"`
	CodePoint string   `json:"codePoint"`
	Category  string   `json:"category"`
	Severity  Severity `json:"severity"`
	Message   string   `json:"message"`
	Excerpt   string   `json:"excerpt,omitempty"`
}

// SkippedFile tracks files skipped during scanning.
type SkippedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Summary is a compact scan summary.
type Summary struct {
	FilesScanned int `json:"filesScanned"`
	FilesSkipped int `json:"filesSkipped"`
	Findings     int `json:"findings"`
}

// Result is the full scan output.
type Result struct {
	Findings     []Finding     `json:"findings"`
	ScannedFiles []string      `json:"scannedFiles"`
	SkippedFiles []SkippedFile `json:"skippedFiles"`
	Summary      Summary       `json:"summary"`
}

// Scan traverses paths recursively and returns all findings.
func Scan(paths []string, opts Options) (Result, error) {
	opts = normalizeOptions(opts)
	if len(paths) == 0 {
		paths = []string{"."}
	}

	cleanPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			cleanPaths = append(cleanPaths, p)
		}
	}
	if len(cleanPaths) == 0 {
		cleanPaths = []string{"."}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return Result{}, err
	}

	res := Result{
		Findings:     []Finding{},
		ScannedFiles: []string{},
		SkippedFiles: []SkippedFile{},
	}
	visited := make(map[string]struct{})

	for _, path := range cleanPaths {
		info, err := os.Stat(path)
		if err != nil {
			return Result{}, err
		}
		if info.IsDir() {
			if err := walkDir(path, cwd, opts, visited, &res); err != nil {
				return Result{}, err
			}
			continue
		}
		if err := scanFile(path, cwd, opts, visited, &res); err != nil {
			return Result{}, err
		}
	}

	sort.Strings(res.ScannedFiles)
	sort.Slice(res.SkippedFiles, func(i, j int) bool {
		return res.SkippedFiles[i].Path < res.SkippedFiles[j].Path
	})
	sort.Slice(res.Findings, func(i, j int) bool {
		a, b := res.Findings[i], res.Findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return a.CodePoint < b.CodePoint
	})

	res.Summary = Summary{
		FilesScanned: len(res.ScannedFiles),
		FilesSkipped: len(res.SkippedFiles),
		Findings:     len(res.Findings),
	}
	return res, nil
}

func normalizeOptions(opts Options) Options {
	if opts.AllowRunes == nil {
		opts.AllowRunes = map[rune]struct{}{}
	}
	if opts.Severity != SeverityWarning {
		opts.Severity = SeverityError
	}
	return opts
}

func walkDir(root, cwd string, opts Options, visited map[string]struct{}, res *Result) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		display := displayPath(cwd, path)
		if d.IsDir() {
			if display != "." && isExcluded(display, opts.Exclude) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return scanFile(path, cwd, opts, visited, res)
	})
}

func scanFile(path, cwd string, opts Options, visited map[string]struct{}, res *Result) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, ok := visited[abs]; ok {
		return nil
	}
	visited[abs] = struct{}{}

	display := displayPath(cwd, abs)
	if !isIncluded(display, opts.Include) {
		return nil
	}
	if isExcluded(display, opts.Exclude) {
		return nil
	}
	if isAllowedFile(display, opts.AllowFilePatterns) {
		res.SkippedFiles = append(res.SkippedFiles, SkippedFile{Path: display, Reason: "allowed by file pattern"})
		return nil
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("read %s: %w", display, err)
	}
	if isBinary(data) {
		res.SkippedFiles = append(res.SkippedFiles, SkippedFile{Path: display, Reason: "binary file"})
		return nil
	}

	res.ScannedFiles = append(res.ScannedFiles, display)
	findings := scanContent(display, data, syntaxForPath(display), opts)
	if len(findings) > 0 {
		res.Findings = append(res.Findings, findings...)
	}
	return nil
}

func isIncluded(path string, include []string) bool {
	if len(include) == 0 {
		return true
	}
	return matches(path, include)
}

func isExcluded(path string, exclude []string) bool {
	if len(exclude) == 0 {
		return false
	}
	if matches(path, exclude) {
		return true
	}
	return matches(path+"/", exclude)
}

func isAllowedFile(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	return matches(path, patterns)
}

func matches(path string, patterns []string) bool {
	norm := filepath.ToSlash(path)
	base := filepath.Base(norm)
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if match.Match(p, norm) || match.Match(p, base) {
			return true
		}
		p = filepath.ToSlash(p)
		if strings.HasSuffix(p, "/**") {
			prefix := strings.TrimSuffix(p, "/**")
			if norm == prefix || strings.HasPrefix(norm, prefix+"/") {
				return true
			}
		}
	}
	return false
}

func displayPath(cwd, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(cwd, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(abs)
	}
	if rel == "." {
		return rel
	}
	return filepath.ToSlash(rel)
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return true
	}
	control := 0
	for _, b := range sample {
		if b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		if b < 0x20 || b == 0x7f {
			control++
		}
	}
	return float64(control)/float64(len(sample)) > 0.30
}

type syntaxRules struct {
	lineComments []string
	blockStart   string
	blockEnd     string
	strings      bool
	backtick     bool
}

func syntaxForPath(path string) syntaxRules {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".java", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".swift", ".kt", ".kts", ".rs", ".php":
		return syntaxRules{lineComments: []string{"//"}, blockStart: "/*", blockEnd: "*/", strings: true, backtick: true}
	case ".py", ".rb", ".sh", ".bash", ".zsh", ".yaml", ".yml", ".toml", ".ini", ".conf", ".properties":
		return syntaxRules{lineComments: []string{"#"}, strings: true}
	case ".sql":
		return syntaxRules{lineComments: []string{"--"}, blockStart: "/*", blockEnd: "*/", strings: true}
	case ".lua":
		return syntaxRules{lineComments: []string{"--"}, strings: true}
	default:
		if base == "dockerfile" || strings.HasSuffix(base, ".dockerfile") {
			return syntaxRules{lineComments: []string{"#"}, strings: true}
		}
		return syntaxRules{}
	}
}

type scanState int

const (
	stateCode scanState = iota
	stateLineComment
	stateBlockComment
	stateSingleString
	stateDoubleString
	stateBacktickString
)

func scanContent(path string, data []byte, syntax syntaxRules, opts Options) []Finding {
	text := string(data)
	lines := strings.Split(text, "\n")
	findings := make([]Finding, 0)
	line := 1
	col := 1
	state := stateCode
	escaped := false

	for i := 0; i < len(text); {
		switch state {
		case stateCode:
			if syntax.blockStart != "" && strings.HasPrefix(text[i:], syntax.blockStart) {
				i, line, col = advanceByToken(i, line, col, syntax.blockStart)
				state = stateBlockComment
				escaped = false
				continue
			}
			if token, ok := matchPrefix(text[i:], syntax.lineComments); ok {
				i, line, col = advanceByToken(i, line, col, token)
				state = stateLineComment
				escaped = false
				continue
			}
			if syntax.strings {
				switch text[i] {
				case '\'':
					i++
					col++
					state = stateSingleString
					escaped = false
					continue
				case '"':
					i++
					col++
					state = stateDoubleString
					escaped = false
					continue
				case '`':
					if syntax.backtick {
						i++
						col++
						state = stateBacktickString
						escaped = false
						continue
					}
				}
			}
		case stateLineComment:
			if text[i] == '\n' {
				i++
				line++
				col = 1
				state = stateCode
				escaped = false
				continue
			}
		case stateBlockComment:
			if syntax.blockEnd != "" && strings.HasPrefix(text[i:], syntax.blockEnd) {
				i, line, col = advanceByToken(i, line, col, syntax.blockEnd)
				state = stateCode
				escaped = false
				continue
			}
		case stateSingleString:
			if !escaped {
				if text[i] == '\\' {
					i++
					col++
					escaped = true
					continue
				}
				if text[i] == '\'' {
					i++
					col++
					state = stateCode
					continue
				}
			}
		case stateDoubleString:
			if !escaped {
				if text[i] == '\\' {
					i++
					col++
					escaped = true
					continue
				}
				if text[i] == '"' {
					i++
					col++
					state = stateCode
					continue
				}
			}
		case stateBacktickString:
			if text[i] == '`' {
				i++
				col++
				state = stateCode
				continue
			}
		}

		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			if shouldInspect(state, opts) {
				findings = append(findings, Finding{
					Path:      path,
					Line:      line,
					Column:    col,
					Character: "?",
					CodePoint: "invalid-utf8",
					Category:  "Invalid UTF-8",
					Severity:  opts.Severity,
					Message:   "Detected invalid UTF-8 byte sequence",
					Excerpt:   lineExcerpt(lines, line),
				})
			}
			i++
			col++
			if escaped {
				escaped = false
			}
			continue
		}

		if shouldInspect(state, opts) && !isAllowedRune(r, opts.AllowRunes) {
			category := categoryForRune(r)
			codePoint := fmt.Sprintf("U+%04X", r)
			findings = append(findings, Finding{
				Path:      path,
				Line:      line,
				Column:    col,
				Character: string(r),
				CodePoint: codePoint,
				Category:  category,
				Severity:  opts.Severity,
				Message:   fmt.Sprintf("Detected %s character %q (%s)", category, string(r), codePoint),
				Excerpt:   lineExcerpt(lines, line),
			})
		}

		i += size
		if r == '\n' {
			line++
			col = 1
			if state == stateLineComment {
				state = stateCode
			}
		} else {
			col++
		}
		if escaped {
			escaped = false
		}
	}

	return findings
}

func matchPrefix(input string, prefixes []string) (string, bool) {
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		if strings.HasPrefix(input, p) {
			return p, true
		}
	}
	return "", false
}

func advanceByToken(i, line, col int, token string) (int, int, int) {
	for _, r := range token {
		i += utf8.RuneLen(r)
		if r == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return i, line, col
}

func shouldInspect(state scanState, opts Options) bool {
	switch state {
	case stateLineComment, stateBlockComment:
		return !opts.IgnoreComments
	case stateSingleString, stateDoubleString, stateBacktickString:
		return !opts.IgnoreStrings
	default:
		return true
	}
}

func isAllowedRune(r rune, allow map[rune]struct{}) bool {
	if r == '\n' || r == '\r' || r == '\t' {
		return true
	}
	if r >= 0x20 && r <= 0x7e {
		return true
	}
	_, ok := allow[r]
	return ok
}

func lineExcerpt(lines []string, line int) string {
	if line < 1 || line > len(lines) {
		return ""
	}
	excerpt := strings.TrimRight(lines[line-1], "\r")
	if len(excerpt) > 160 {
		return excerpt[:160] + "..."
	}
	return excerpt
}

func categoryForRune(r rune) string {
	switch {
	case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
		return "CJK"
	case unicode.In(r, unicode.Cyrillic):
		return "Cyrillic"
	case unicode.In(r, unicode.Arabic):
		return "Arabic"
	case unicode.In(r, unicode.Thai):
		return "Thai"
	case unicode.In(r, unicode.Devanagari):
		return "Devanagari"
	case unicode.In(r, unicode.Hebrew):
		return "Hebrew"
	case unicode.In(r, unicode.Greek):
		return "Greek"
	case unicode.In(r, unicode.Latin):
		return "Latin Extended"
	case unicode.IsPunct(r) || unicode.IsSymbol(r):
		return "Unicode Symbol"
	default:
		return "Other Unicode"
	}
}
