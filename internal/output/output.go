package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/TT-AIXion/englint/internal/scanner"
)

const fixSuggestion = "Auto-fix is not implemented yet. Replace characters manually or add safe symbols to the allow list in .englint.yaml."

// ScanOptions controls printed details.
type ScanOptions struct {
	Verbose      bool
	FixRequested bool
}

// Writer renders scan output in JSON or human-readable mode.
type Writer struct {
	JSON    bool
	NoColor bool
	Out     io.Writer
	ErrW    io.Writer
}

func New(jsonMode, noColor bool, out, errW io.Writer) Writer {
	if out == nil {
		out = os.Stdout
	}
	if errW == nil {
		errW = os.Stderr
	}
	return Writer{JSON: jsonMode, NoColor: noColor, Out: out, ErrW: errW}
}

func (w Writer) PrintScan(result scanner.Result, opts ScanOptions) error {
	if w.JSON {
		return w.printScanJSON(result, opts)
	}
	return w.printScanHuman(result, opts)
}

func (w Writer) printScanJSON(result scanner.Result, opts ScanOptions) error {
	payload := struct {
		Summary      scanner.Summary       `json:"summary"`
		Findings     []scanner.Finding     `json:"findings"`
		Scanned      []string              `json:"scannedFiles,omitempty"`
		Skipped      []scanner.SkippedFile `json:"skippedFiles,omitempty"`
		FixSuggested string                `json:"fixSuggested,omitempty"`
	}{
		Summary:  result.Summary,
		Findings: result.Findings,
		Scanned:  result.ScannedFiles,
		Skipped:  result.SkippedFiles,
	}
	if opts.FixRequested && result.Summary.Findings > 0 {
		payload.FixSuggested = fixSuggestion
	}
	enc := json.NewEncoder(w.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func (w Writer) printScanHuman(result scanner.Result, opts ScanOptions) error {
	if opts.Verbose {
		for _, file := range result.ScannedFiles {
			if _, err := fmt.Fprintf(w.Out, "SCANNED %s\n", file); err != nil {
				return err
			}
		}
		for _, skipped := range result.SkippedFiles {
			if _, err := fmt.Fprintf(w.Out, "SKIPPED %s (%s)\n", skipped.Path, skipped.Reason); err != nil {
				return err
			}
		}
	}

	for _, finding := range result.Findings {
		label := strings.ToUpper(string(finding.Severity))
		label = w.colorize(label, finding.Severity)
		if _, err := fmt.Fprintf(
			w.Out,
			"%s %s:%d:%d [%s] %s (%s)\n",
			label,
			finding.Path,
			finding.Line,
			finding.Column,
			finding.Category,
			finding.Character,
			finding.CodePoint,
		); err != nil {
			return err
		}
		if strings.TrimSpace(finding.Excerpt) != "" {
			if _, err := fmt.Fprintf(w.Out, "  %s\n", finding.Excerpt); err != nil {
				return err
			}
		}
	}

	if result.Summary.Findings == 0 {
		if _, err := fmt.Fprintln(w.Out, "No non-English text found."); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(
		w.Out,
		"Summary: scanned=%d skipped=%d findings=%d\n",
		result.Summary.FilesScanned,
		result.Summary.FilesSkipped,
		result.Summary.Findings,
	); err != nil {
		return err
	}

	if opts.FixRequested && result.Summary.Findings > 0 {
		if _, err := fmt.Fprintln(w.Out, fixSuggestion); err != nil {
			return err
		}
	}
	return nil
}

func (w Writer) colorize(label string, severity scanner.Severity) string {
	if w.NoColor {
		return label
	}
	switch severity {
	case scanner.SeverityWarning:
		return "\x1b[33m" + label + "\x1b[0m"
	default:
		return "\x1b[31m" + label + "\x1b[0m"
	}
}
