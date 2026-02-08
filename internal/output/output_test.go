package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/TT-AIXion/englint/internal/scanner"
)

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failure")
}

type failAtWriter struct {
	count  int
	failAt int
}

func (w *failAtWriter) Write(p []byte) (int, error) {
	w.count++
	if w.count == w.failAt {
		return 0, errors.New("forced write failure")
	}
	return len(p), nil
}

func TestPrintScanHuman(t *testing.T) {
	var out bytes.Buffer
	w := New(false, true, &out, &out)
	result := scanner.Result{
		Findings: []scanner.Finding{
			{
				Path:      "a.go",
				Line:      3,
				Column:    7,
				Character: "あ",
				CodePoint: "U+3042",
				Category:  "CJK",
				Severity:  scanner.SeverityError,
				Excerpt:   "var s = \"あ\"",
			},
		},
		ScannedFiles: []string{"a.go"},
		SkippedFiles: []scanner.SkippedFile{{Path: "b.bin", Reason: "binary file"}},
		Summary:      scanner.Summary{FilesScanned: 1, FilesSkipped: 1, Findings: 1},
	}

	if err := w.PrintScan(result, ScanOptions{Verbose: true, FixRequested: true}); err != nil {
		t.Fatalf("PrintScan returned error: %v", err)
	}
	text := out.String()
	for _, mustContain := range []string{
		"SCANNED a.go",
		"SKIPPED b.bin (binary file)",
		"ERROR a.go:3:7 [CJK]",
		"Summary: scanned=1 skipped=1 findings=1",
		"Auto-fix is not implemented yet.",
	} {
		if !strings.Contains(text, mustContain) {
			t.Fatalf("expected output to contain %q\nactual:\n%s", mustContain, text)
		}
	}
}

func TestPrintScanHumanNoFindings(t *testing.T) {
	var out bytes.Buffer
	w := New(false, false, &out, &out)
	result := scanner.Result{Summary: scanner.Summary{FilesScanned: 2}}
	if err := w.PrintScan(result, ScanOptions{}); err != nil {
		t.Fatalf("PrintScan returned error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "No non-English text found.") {
		t.Fatalf("expected no-findings message")
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("unexpected color code when no finding lines")
	}
}

func TestPrintScanJSON(t *testing.T) {
	var out bytes.Buffer
	w := New(true, true, &out, &out)
	result := scanner.Result{
		Findings: []scanner.Finding{{Path: "a.go", Severity: scanner.SeverityWarning}},
		Summary:  scanner.Summary{Findings: 1},
	}
	if err := w.PrintScan(result, ScanOptions{FixRequested: true}); err != nil {
		t.Fatalf("PrintScan returned error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if payload["fixSuggested"] == "" {
		t.Fatalf("expected fix suggestion in json output")
	}
	if payload["summary"] == nil {
		t.Fatalf("expected summary in json output")
	}
}

func TestPrintScanWriterErrors(t *testing.T) {
	result := scanner.Result{
		Findings:     []scanner.Finding{{Path: "a.go", Severity: scanner.SeverityError, Category: "CJK", Character: "あ", CodePoint: "U+3042"}},
		ScannedFiles: []string{"a.go"},
		SkippedFiles: []scanner.SkippedFile{{Path: "b.bin", Reason: "binary"}},
		Summary:      scanner.Summary{FilesScanned: 1, FilesSkipped: 1, Findings: 1},
	}

	t.Run("json encode error", func(t *testing.T) {
		w := New(true, true, errWriter{}, errWriter{})
		if err := w.PrintScan(result, ScanOptions{}); err == nil {
			t.Fatalf("expected json output error")
		}
	})

	t.Run("human verbose write error", func(t *testing.T) {
		w := New(false, true, errWriter{}, errWriter{})
		if err := w.PrintScan(result, ScanOptions{Verbose: true}); err == nil {
			t.Fatalf("expected human output error")
		}
	})

	t.Run("human excerpt write error", func(t *testing.T) {
		fw := &failAtWriter{failAt: 2}
		w := New(false, true, fw, fw)
		res := scanner.Result{
			Findings: []scanner.Finding{{
				Path:      "a.go",
				Line:      1,
				Column:    1,
				Character: "あ",
				CodePoint: "U+3042",
				Category:  "CJK",
				Severity:  scanner.SeverityError,
				Excerpt:   "var _ = \"あ\"",
			}},
			Summary: scanner.Summary{FilesScanned: 1, Findings: 1},
		}
		if err := w.PrintScan(res, ScanOptions{}); err == nil {
			t.Fatalf("expected excerpt write error")
		}
	})

	t.Run("human no-findings message error", func(t *testing.T) {
		fw := &failAtWriter{failAt: 1}
		w := New(false, true, fw, fw)
		if err := w.PrintScan(scanner.Result{}, ScanOptions{}); err == nil {
			t.Fatalf("expected no-findings write error")
		}
	})

	t.Run("human summary error", func(t *testing.T) {
		fw := &failAtWriter{failAt: 2}
		w := New(false, true, fw, fw)
		res := scanner.Result{
			Findings: []scanner.Finding{{Path: "a.go", Character: "あ", CodePoint: "U+3042", Category: "CJK", Severity: scanner.SeverityError}},
			Summary:  scanner.Summary{FilesScanned: 1, Findings: 1},
		}
		if err := w.PrintScan(res, ScanOptions{}); err == nil {
			t.Fatalf("expected summary write error")
		}
	})

	t.Run("human fix message error", func(t *testing.T) {
		fw := &failAtWriter{failAt: 3}
		w := New(false, true, fw, fw)
		res := scanner.Result{
			Findings: []scanner.Finding{{Path: "a.go", Character: "あ", CodePoint: "U+3042", Category: "CJK", Severity: scanner.SeverityError}},
			Summary:  scanner.Summary{FilesScanned: 1, Findings: 1},
		}
		if err := w.PrintScan(res, ScanOptions{FixRequested: true}); err == nil {
			t.Fatalf("expected fix message write error")
		}
	})
}

func TestNewDefaultsAndColorize(t *testing.T) {
	w := New(false, false, nil, nil)
	if w.Out == nil || w.ErrW == nil {
		t.Fatalf("expected stdio defaults")
	}

	errColored := w.colorize("ERROR", scanner.SeverityError)
	if !strings.Contains(errColored, "\x1b[31m") {
		t.Fatalf("expected red color for error")
	}
	warnColored := w.colorize("WARNING", scanner.SeverityWarning)
	if !strings.Contains(warnColored, "\x1b[33m") {
		t.Fatalf("expected yellow color for warning")
	}

	plain := New(false, true, &bytes.Buffer{}, &bytes.Buffer{}).colorize("ERROR", scanner.SeverityError)
	if plain != "ERROR" {
		t.Fatalf("expected plain label without color")
	}
}
