package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/TT-AIXion/englint/internal/config"
	"github.com/TT-AIXion/englint/internal/output"
	"github.com/TT-AIXion/englint/internal/scanner"
)

var Version = "dev"
var exitFunc = os.Exit

func main() {
	exitFunc(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

func runMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version":
		_, _ = fmt.Fprintf(stdout, "englint %s\n", Version)
		return 0
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "scan":
		return runScan(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		printUsage(stderr)
		return 1
	}
}

type scanArgs struct {
	ConfigPath string
	Include    []string
	Exclude    []string
	JSON       bool
	Fix        bool
	Severity   string
	NoColor    bool
	Verbose    bool
	Paths      []string
}

func parseScanArgs(args []string) (scanArgs, error) {
	out := scanArgs{ConfigPath: ".englint.yaml"}
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if arg == "--" {
			out.Paths = append(out.Paths, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			out.Paths = append(out.Paths, arg)
			continue
		}

		switch {
		case arg == "--json":
			out.JSON = true
		case arg == "--fix":
			out.Fix = true
		case arg == "--no-color":
			out.NoColor = true
		case arg == "--verbose":
			out.Verbose = true
		case arg == "--config":
			if i+1 >= len(args) {
				return scanArgs{}, fmt.Errorf("flag --config requires a value")
			}
			i++
			out.ConfigPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			out.ConfigPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--exclude":
			if i+1 >= len(args) {
				return scanArgs{}, fmt.Errorf("flag --exclude requires a value")
			}
			i++
			out.Exclude = append(out.Exclude, args[i])
		case strings.HasPrefix(arg, "--exclude="):
			out.Exclude = append(out.Exclude, strings.TrimPrefix(arg, "--exclude="))
		case arg == "--include":
			if i+1 >= len(args) {
				return scanArgs{}, fmt.Errorf("flag --include requires a value")
			}
			i++
			out.Include = append(out.Include, args[i])
		case strings.HasPrefix(arg, "--include="):
			out.Include = append(out.Include, strings.TrimPrefix(arg, "--include="))
		case arg == "--severity":
			if i+1 >= len(args) {
				return scanArgs{}, fmt.Errorf("flag --severity requires a value")
			}
			i++
			out.Severity = args[i]
		case strings.HasPrefix(arg, "--severity="):
			out.Severity = strings.TrimPrefix(arg, "--severity=")
		default:
			return scanArgs{}, fmt.Errorf("unknown flag: %s", arg)
		}
	}

	if len(out.Paths) == 0 {
		out.Paths = []string{"."}
	}
	if strings.TrimSpace(out.ConfigPath) == "" {
		out.ConfigPath = ".englint.yaml"
	}
	out.Severity = strings.ToLower(strings.TrimSpace(out.Severity))
	return out, nil
}

type initArgs struct {
	ConfigPath string
}

func parseInitArgs(args []string) (initArgs, error) {
	out := initArgs{ConfigPath: ".englint.yaml"}
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		switch {
		case arg == "--config":
			if i+1 >= len(args) {
				return initArgs{}, fmt.Errorf("flag --config requires a value")
			}
			i++
			out.ConfigPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			out.ConfigPath = strings.TrimPrefix(arg, "--config=")
		default:
			return initArgs{}, fmt.Errorf("unknown flag for init: %s", arg)
		}
	}
	if strings.TrimSpace(out.ConfigPath) == "" {
		out.ConfigPath = ".englint.yaml"
	}
	return out, nil
}

func runScan(args []string, stdout, stderr io.Writer) int {
	parsed, err := parseScanArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "scan argument error: %v\n", err)
		printScanUsage(stderr)
		return 1
	}

	cfg, err := config.Load(parsed.ConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config error: %v\n", err)
		return 1
	}

	cfg.Include = append(cfg.Include, parsed.Include...)
	cfg.Exclude = append(cfg.Exclude, parsed.Exclude...)
	if parsed.Severity != "" {
		cfg.Severity = parsed.Severity
	}
	cfg = config.ApplyDefaults(cfg)
	if err := config.Validate(cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "config validation error: %v\n", err)
		return 1
	}

	sev := scanner.SeverityError
	if cfg.Severity == config.SeverityWarning {
		sev = scanner.SeverityWarning
	}

	result, err := scanner.Scan(parsed.Paths, scanner.Options{
		Include:           cfg.Include,
		Exclude:           cfg.Exclude,
		AllowRunes:        config.AllowedRuneMap(cfg.Allow),
		Severity:          sev,
		IgnoreComments:    cfg.IgnoreComments,
		IgnoreStrings:     cfg.IgnoreStrings,
		AllowFilePatterns: cfg.AllowFilePatterns,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "scan error: %v\n", err)
		return 1
	}

	writer := output.New(parsed.JSON, parsed.NoColor || os.Getenv("NO_COLOR") != "", stdout, stderr)
	if err := writer.PrintScan(result, output.ScanOptions{Verbose: parsed.Verbose, FixRequested: parsed.Fix}); err != nil {
		_, _ = fmt.Fprintf(stderr, "output error: %v\n", err)
		return 1
	}
	if result.Summary.Findings > 0 {
		return 1
	}
	return 0
}

func runInit(args []string, stdout, stderr io.Writer) int {
	parsed, err := parseInitArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init argument error: %v\n", err)
		return 1
	}
	if _, err := os.Stat(parsed.ConfigPath); err == nil {
		_, _ = fmt.Fprintf(stderr, "config file already exists: %s\n", parsed.ConfigPath)
		return 1
	} else if !os.IsNotExist(err) {
		_, _ = fmt.Fprintf(stderr, "failed to check config file: %v\n", err)
		return 1
	}
	if err := config.WriteDefault(parsed.ConfigPath); err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to create config: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Created %s\n", parsed.ConfigPath)
	return 0
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "englint - detect non-English text in source files")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  englint scan [paths...] [flags]")
	_, _ = fmt.Fprintln(w, "  englint init [--config <path>]")
	_, _ = fmt.Fprintln(w, "  englint version")
	_, _ = fmt.Fprintln(w, "")
	printScanUsage(w)
}

func printScanUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Scan flags:")
	_, _ = fmt.Fprintln(w, "  --config <path>          Config file path (default: .englint.yaml)")
	_, _ = fmt.Fprintln(w, "  --exclude <glob>         Exclude glob pattern (repeatable)")
	_, _ = fmt.Fprintln(w, "  --include <glob>         Include glob pattern (repeatable)")
	_, _ = fmt.Fprintln(w, "  --json                   JSON output")
	_, _ = fmt.Fprintln(w, "  --fix                    Auto-fix placeholder mode")
	_, _ = fmt.Fprintln(w, "  --severity <level>       Default severity: error|warning")
	_, _ = fmt.Fprintln(w, "  --no-color               Disable color output")
	_, _ = fmt.Fprintln(w, "  --verbose                Show all scanned and skipped files")
}
