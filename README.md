# englint

`englint` is a CLI linter that detects non-English text in source files.

## Why englint

- CI-friendly exit code (`1` when non-English text is detected)
- Recursive directory scanning
- Include and exclude glob patterns
- Unicode category detection (CJK, Cyrillic, Arabic, Thai, and more)
- Configurable allow list and context exceptions
- Human-readable and JSON output

## Install

Homebrew:

```sh
brew install TT-AIXion/englint/englint
```

Go install:

```sh
go install github.com/TT-AIXion/englint/cmd/englint@latest
```

Build from source:

```sh
go build -o englint ./cmd/englint
```

## Quick Start

Initialize config:

```sh
englint init
```

Scan current directory:

```sh
englint scan .
```

Scan with JSON output:

```sh
englint scan . --json
```

## Commands

```text
englint scan [paths...] [flags]
englint init
englint version
```

## Scan Flags

- `--config <path>`: config file path (default: `.englint.yaml`)
- `--exclude <glob>`: exclude glob (repeatable)
- `--include <glob>`: include glob (repeatable)
- `--json`: JSON output
- `--fix`: auto-fix placeholder mode
- `--severity <error|warning>`: default severity
- `--no-color`: disable color output
- `--verbose`: print scanned and skipped files

## Configuration

Default `.englint.yaml`:

```yaml
include:
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
  - "©"
  - "→"
severity: error
```

Optional keys:

- `ignore_comments`: ignore non-English text in comments
- `ignore_strings`: ignore non-English text in string literals
- `allow_file_patterns`: glob patterns where non-English text is allowed

## Output Examples

Human-readable:

```text
ERROR src/app.go:12:9 [CJK] <char> (U+65E5)
Summary: scanned=12 skipped=1 findings=1
```

JSON:

```json
{
  "summary": {
    "filesScanned": 12,
    "filesSkipped": 1,
    "findings": 1
  },
  "findings": [
    {
      "path": "src/app.go",
      "line": 12,
      "column": 9,
      "character": "\\u65E5",
      "codePoint": "U+65E5",
      "category": "CJK",
      "severity": "error"
    }
  ]
}
```

## Development

```sh
gofmt -w .
go vet ./...
go test ./...
```

## License

MIT. See `LICENSE`.
