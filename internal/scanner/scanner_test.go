package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDetectsUnicodeCategories(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		wantCategory string
	}{
		{name: "cjk", file: "japanese.go", wantCategory: "CJK"},
		{name: "cyrillic", file: "cyrillic.txt", wantCategory: "Cyrillic"},
		{name: "arabic", file: "arabic.txt", wantCategory: "Arabic"},
		{name: "thai", file: "thai.txt", wantCategory: "Thai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Scan([]string{filepath.Join("testdata", "fixtures", tt.file)}, Options{
				Include:    []string{"**/*"},
				Severity:   SeverityError,
				AllowRunes: map[rune]struct{}{},
			})
			if err != nil {
				t.Fatalf("Scan returned error: %v", err)
			}
			if len(res.Findings) == 0 {
				t.Fatalf("expected findings")
			}
			if res.Findings[0].Category != tt.wantCategory {
				t.Fatalf("expected category %q, got %q", tt.wantCategory, res.Findings[0].Category)
			}
		})
	}
}

func TestScanIgnoreCommentsAndStrings(t *testing.T) {
	path := filepath.Join("testdata", "fixtures", "string_comment.go")

	base, err := Scan([]string{path}, Options{Include: []string{"**/*.go"}, Severity: SeverityError})
	if err != nil {
		t.Fatalf("scan base: %v", err)
	}
	if len(base.Findings) == 0 {
		t.Fatalf("expected findings without ignore flags")
	}

	ignored, err := Scan([]string{path}, Options{
		Include:        []string{"**/*.go"},
		Severity:       SeverityError,
		IgnoreComments: true,
		IgnoreStrings:  true,
	})
	if err != nil {
		t.Fatalf("scan ignored: %v", err)
	}
	if len(ignored.Findings) != 0 {
		t.Fatalf("expected no findings when comments/strings are ignored, got %d", len(ignored.Findings))
	}
}

func TestScanAllowRunes(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a.go")
	content := "package p\n\nvar _ = \"©→あ\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	res, err := Scan([]string{path}, Options{
		Include:    []string{"**/*.go"},
		Severity:   SeverityError,
		AllowRunes: map[rune]struct{}{'©': {}, '→': {}},
	})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(res.Findings))
	}
	if res.Findings[0].Character != "あ" {
		t.Fatalf("expected remaining rune to be あ, got %q", res.Findings[0].Character)
	}
}

func TestScanIncludeExclude(t *testing.T) {
	tmp := t.TempDir()
	goFile := filepath.Join(tmp, "a.go")
	txtFile := filepath.Join(tmp, "b.txt")
	if err := os.WriteFile(goFile, []byte("package p\nvar _ = \"こんにちは\"\n"), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}
	if err := os.WriteFile(txtFile, []byte("مرحبا\n"), 0o644); err != nil {
		t.Fatalf("write text file: %v", err)
	}

	res, err := Scan([]string{tmp}, Options{
		Include:  []string{"**/*.go"},
		Exclude:  []string{"**/a.go"},
		Severity: SeverityError,
	})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.ScannedFiles) != 0 {
		t.Fatalf("expected no scanned files after include/exclude, got %v", res.ScannedFiles)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("expected no findings")
	}
}

func TestScanBinaryAndEmpty(t *testing.T) {
	binaryPath := filepath.Join("testdata", "fixtures", "binary.bin")
	emptyPath := filepath.Join("testdata", "fixtures", "empty.txt")

	res, err := Scan([]string{binaryPath, emptyPath}, Options{Include: []string{"**/*"}, Severity: SeverityError})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.SkippedFiles) == 0 {
		t.Fatalf("expected skipped binary file")
	}
	if res.Summary.FilesScanned != 1 {
		t.Fatalf("expected one scanned text file, got %d", res.Summary.FilesScanned)
	}
}

func TestScanAllowedFilePattern(t *testing.T) {
	path := filepath.Join("testdata", "fixtures", "japanese.go")
	res, err := Scan([]string{path}, Options{
		Include:           []string{"**/*.go"},
		Severity:          SeverityError,
		AllowFilePatterns: []string{"**/japanese.go"},
	})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("expected no findings for allowed file pattern")
	}
	if len(res.SkippedFiles) != 1 || res.SkippedFiles[0].Reason != "allowed by file pattern" {
		t.Fatalf("unexpected skipped files: %+v", res.SkippedFiles)
	}
}

func TestScanInvalidUTF8(t *testing.T) {
	path := filepath.Join("testdata", "fixtures", "invalid_utf8.txt")
	res, err := Scan([]string{path}, Options{Include: []string{"**/*"}, Severity: SeverityError})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.Findings) == 0 {
		t.Fatalf("expected finding for invalid utf8")
	}
	if res.Findings[0].Category != "Invalid UTF-8" {
		t.Fatalf("unexpected category: %q", res.Findings[0].Category)
	}
}

func TestScanErrorCases(t *testing.T) {
	if _, err := Scan([]string{"does-not-exist"}, Options{Include: []string{"**/*"}}); err == nil {
		t.Fatalf("expected stat error")
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "a.go")
	if err := os.WriteFile(path, []byte("package p\nvar _ = \"こんにちは\"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	res, err := Scan([]string{path, path}, Options{Include: []string{"**/*.go"}, Severity: SeverityWarning})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if res.Summary.FilesScanned != 1 {
		t.Fatalf("expected deduplicated scan, got %d", res.Summary.FilesScanned)
	}
	if len(res.Findings) == 0 || res.Findings[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity in findings")
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("display path", func(t *testing.T) {
		cwd := t.TempDir()
		abs := filepath.Join(cwd, "a", "b.go")
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if got := displayPath(cwd, abs); got != "a/b.go" {
			t.Fatalf("unexpected display path: %q", got)
		}
		if got := displayPath(cwd, "/tmp/nope/abc.go"); got == "" {
			t.Fatalf("expected non-empty path")
		}
	})

	t.Run("matches and include exclude", func(t *testing.T) {
		if !matches("dir/a.lock", []string{"*.lock"}) {
			t.Fatalf("expected basename match")
		}
		if !isIncluded("a.go", nil) {
			t.Fatalf("nil include should include")
		}
		if isExcluded("src/a.go", nil) {
			t.Fatalf("nil exclude should not exclude")
		}
		if !isExcluded("vendor/pkg/a.go", []string{"vendor/**"}) {
			t.Fatalf("expected excluded path")
		}
		if !isAllowedFile("docs/readme.md", []string{"docs/**"}) {
			t.Fatalf("expected allowed file pattern match")
		}
	})

	t.Run("syntax detection", func(t *testing.T) {
		if s := syntaxForPath("a.go"); len(s.lineComments) == 0 || !s.strings {
			t.Fatalf("unexpected go syntax: %+v", s)
		}
		if s := syntaxForPath("a.sql"); s.blockStart != "/*" {
			t.Fatalf("unexpected sql syntax: %+v", s)
		}
		if s := syntaxForPath("Dockerfile"); len(s.lineComments) == 0 {
			t.Fatalf("unexpected dockerfile syntax: %+v", s)
		}
		if s := syntaxForPath("a.md"); s.strings {
			t.Fatalf("unexpected markdown syntax: %+v", s)
		}
	})

	t.Run("token helpers", func(t *testing.T) {
		if tok, ok := matchPrefix("// abc", []string{"#", "//"}); !ok || tok != "//" {
			t.Fatalf("unexpected token match: %q %v", tok, ok)
		}
		if _, ok := matchPrefix("abc", []string{"//"}); ok {
			t.Fatalf("unexpected token match")
		}
		i, line, col := advanceByToken(0, 1, 1, "/*")
		if i != 2 || line != 1 || col != 3 {
			t.Fatalf("unexpected advance result: %d %d %d", i, line, col)
		}
		if !shouldInspect(stateCode, Options{}) {
			t.Fatalf("code should be inspected")
		}
		if shouldInspect(stateLineComment, Options{IgnoreComments: true}) {
			t.Fatalf("comment should be skipped")
		}
		if shouldInspect(stateDoubleString, Options{IgnoreStrings: true}) {
			t.Fatalf("string should be skipped")
		}
	})

	t.Run("rune and category helpers", func(t *testing.T) {
		if !isAllowedRune('A', nil) || !isAllowedRune('\n', nil) {
			t.Fatalf("ascii printable and whitespace must be allowed")
		}
		if isAllowedRune('あ', nil) {
			t.Fatalf("non-ascii should not be allowed by default")
		}
		if !isAllowedRune('あ', map[rune]struct{}{'あ': {}}) {
			t.Fatalf("allowed rune map should be respected")
		}

		cases := map[rune]string{
			'あ': "CJK",
			'Я': "Cyrillic",
			'ع': "Arabic",
			'ไ': "Thai",
			'अ': "Devanagari",
			'א': "Hebrew",
			'Ω': "Greek",
			'é': "Latin Extended",
			'→': "Unicode Symbol",
		}
		for r, want := range cases {
			if got := categoryForRune(r); got != want {
				t.Fatalf("categoryForRune(%q) = %q, want %q", r, got, want)
			}
		}
	})

	t.Run("line excerpt and binary", func(t *testing.T) {
		if got := lineExcerpt([]string{"a", "b"}, 2); got != "b" {
			t.Fatalf("unexpected excerpt: %q", got)
		}
		if got := lineExcerpt([]string{"a"}, 10); got != "" {
			t.Fatalf("expected empty excerpt for out of range")
		}
		long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		if got := lineExcerpt([]string{long}, 1); len(got) <= 160 {
			t.Fatalf("expected truncated excerpt")
		}

		if isBinary([]byte{}) {
			t.Fatalf("empty data should not be binary")
		}
		if !isBinary([]byte{0x00, 0x01}) {
			t.Fatalf("nul bytes should be binary")
		}
		if isBinary([]byte("hello\nworld\n")) {
			t.Fatalf("plain text should not be binary")
		}
	})
}

func TestScanNormalizeAndDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a.go")
	if err := os.WriteFile(path, []byte("package p\nvar _ = \"こんにちは\"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	res, err := Scan([]string{"  ", path}, Options{})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.Findings) == 0 {
		t.Fatalf("expected finding with default options")
	}
	if res.Findings[0].Severity != SeverityError {
		t.Fatalf("expected default severity error")
	}
}

func TestScanContentStateMachineCoverage(t *testing.T) {
	syntax := syntaxRules{
		lineComments: []string{"//"},
		blockStart:   "/*",
		blockEnd:     "*/",
		strings:      true,
		backtick:     true,
	}
	text := "" +
		"漢\n" +
		"// Коммент\n" +
		"/* تعليق */\n" +
		"var a = 'ไทย\\''\n" +
		"var b = \"Я\\\"Я\"\n" +
		"var c = `אב`\n"

	all := scanContent("sample.go", []byte(text), syntax, Options{Severity: SeverityError})
	if len(all) == 0 {
		t.Fatalf("expected findings")
	}

	ignored := scanContent("sample.go", []byte(text), syntax, Options{
		Severity:       SeverityError,
		IgnoreComments: true,
		IgnoreStrings:  true,
	})
	if len(ignored) != 1 {
		t.Fatalf("expected only code finding when comments and strings are ignored, got %d", len(ignored))
	}
	if ignored[0].Character != "漢" {
		t.Fatalf("expected remaining finding to be code rune")
	}
}

func TestScanContentAdditionalBranches(t *testing.T) {
	t.Run("invalid utf8 ignored in comments", func(t *testing.T) {
		syntax := syntaxRules{lineComments: []string{"//"}}
		data := []byte("// \xff\xfe\n")
		findings := scanContent("a.go", data, syntax, Options{IgnoreComments: true, Severity: SeverityError})
		if len(findings) != 0 {
			t.Fatalf("expected no findings when comment scanning is disabled")
		}
	})

	t.Run("backtick unsupported path", func(t *testing.T) {
		syntax := syntaxRules{strings: true, backtick: false}
		findings := scanContent("a.txt", []byte("`é`\n"), syntax, Options{Severity: SeverityError})
		if len(findings) == 0 {
			t.Fatalf("expected finding when backtick is plain text")
		}
	})

	t.Run("other unicode category", func(t *testing.T) {
		if got := categoryForRune('𐍈'); got != "Other Unicode" {
			t.Fatalf("unexpected category: %q", got)
		}
	})
}

func TestScanFilesystemBranches(t *testing.T) {
	t.Run("excluded directory skipped in walk", func(t *testing.T) {
		tmp := t.TempDir()
		excludedDir := filepath.Join(tmp, "vendor")
		if err := os.MkdirAll(excludedDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		file := filepath.Join(excludedDir, "bad.go")
		if err := os.WriteFile(file, []byte("package p\nvar _ = \"こんにちは\"\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		// Paths outside cwd are reported as absolute, so use recursive exclusion.
		res, err := Scan([]string{tmp}, Options{Include: []string{"**/*.go"}, Exclude: []string{"**/vendor/**"}, Severity: SeverityError})
		if err != nil {
			t.Fatalf("scan error: %v", err)
		}
		if len(res.Findings) != 0 {
			t.Fatalf("expected excluded directory to be skipped")
		}
	})

	t.Run("read error from unreadable file", func(t *testing.T) {
		tmp := t.TempDir()
		file := filepath.Join(tmp, "bad.go")
		if err := os.WriteFile(file, []byte("package p\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.Chmod(file, 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		defer os.Chmod(file, 0o644)
		if _, err := Scan([]string{file}, Options{Include: []string{"**/*.go"}}); err == nil {
			t.Fatalf("expected read error")
		}
	})

	t.Run("walk error on unreadable directory", func(t *testing.T) {
		tmp := t.TempDir()
		dir := filepath.Join(tmp, "blocked")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Chmod(dir, 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		defer os.Chmod(dir, 0o700)
		if _, err := Scan([]string{dir}, Options{Include: []string{"**/*"}}); err == nil {
			t.Fatalf("expected walk error")
		}
	})

	t.Run("skip non-regular files", func(t *testing.T) {
		tmp := t.TempDir()
		target := filepath.Join(tmp, "target.go")
		link := filepath.Join(tmp, "link.go")
		if err := os.WriteFile(target, []byte("package p\nvar _ = \"こんにちは\"\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		res, err := Scan([]string{tmp}, Options{Include: []string{"**/*.go"}, Severity: SeverityError})
		if err != nil {
			t.Fatalf("scan error: %v", err)
		}
		if len(res.ScannedFiles) != 1 {
			t.Fatalf("expected only regular file to be scanned, got %v", res.ScannedFiles)
		}
	})
}

func TestAdditionalHelpers(t *testing.T) {
	t.Run("advance token newline and rune len", func(t *testing.T) {
		i, line, col := advanceByToken(0, 1, 1, "a\nβ")
		if i <= 0 || line != 2 || col <= 1 {
			t.Fatalf("unexpected advance result: %d %d %d", i, line, col)
		}
	})

	t.Run("binary ratio branch", func(t *testing.T) {
		data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		if !isBinary(data) {
			t.Fatalf("expected control-heavy bytes to be binary")
		}
	})

	t.Run("syntax lua", func(t *testing.T) {
		s := syntaxForPath("script.lua")
		if len(s.lineComments) == 0 {
			t.Fatalf("expected lua comment syntax")
		}
	})

	t.Run("empty patterns in matches", func(t *testing.T) {
		if matches("a.go", []string{"", " "}) {
			t.Fatalf("expected no match for blank patterns")
		}
	})

	t.Run("display path abs error path", func(t *testing.T) {
		if got := displayPath(".", string([]byte{0})); got == "" {
			t.Fatalf("expected fallback path on abs error")
		}
	})

	t.Run("match prefix blank entries", func(t *testing.T) {
		if _, ok := matchPrefix("abc", []string{"", " "}); ok {
			t.Fatalf("expected no token match")
		}
	})
}

func TestScanGetwdError(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	removed := filepath.Join(tmp, "removed")
	if err := os.MkdirAll(removed, 0o755); err != nil {
		t.Fatalf("mkdir removed: %v", err)
	}
	if err := os.Chdir(removed); err != nil {
		t.Fatalf("chdir removed: %v", err)
	}
	if err := os.RemoveAll(removed); err != nil {
		_ = os.Chdir(origWD)
		t.Skipf("unable to remove active directory on this platform: %v", err)
	}
	defer os.Chdir(origWD)

	if _, err := Scan([]string{"."}, Options{}); err == nil {
		t.Skip("platform kept working directory resolvable after removal")
	}
}
