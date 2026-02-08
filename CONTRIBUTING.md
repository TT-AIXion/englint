# Contributing

Thanks for considering a contribution to `englint`.

## Development

Requirements:

- Go 1.22+
- Git
- macOS or Linux

Run checks:

```sh
gofmt -w .
go vet ./...
go test ./...
golangci-lint run
govulncheck ./...
```

## Style

- `gofmt` is required
- Keep names explicit and simple
- Keep dependencies minimal
- Prefer table-driven tests for behavior changes

## Issues

- Use Issues for bugs and feature requests
- For security reports, use `SECURITY.md`

## Pull Requests

- Keep changes focused and reviewable
- Add or update tests for behavior changes
- Use Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`)
