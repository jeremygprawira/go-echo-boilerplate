# Task Completion Checklist

After completing any coding task, run the following in order:

## 1. Build Check (mandatory)
```bash
go build ./...
```
Must produce zero errors before anything else.

## 2. Vet (mandatory)
```bash
go vet ./...
```
Fix any issues reported.

## 3. Tests (mandatory for touched packages)
```bash
go test -race ./...
# Or for a specific package:
go test -race -v ./internal/pkg/stringc/...
```

## 4. Lint (recommended)
```bash
make lint
# or if golangci-lint not installed:
golangci-lint run ./...
```

## 5. Docs (if handlers or models changed)
```bash
make docs
```

## 6. Module hygiene (if go.mod / imports changed)
```bash
make tidy
```

## Key Invariants to Verify
- No raw errors leaked to HTTP responses (all go through `response.Error`)
- No `logger.AddError` calls in service/repository layers (only in `response.Error`)
- No magic PostgreSQL error code strings (use `pgUniqueViolation` constant)
- GORM log mode respects `config.Application.Environment`
- All new exported functions have godoc comments
- `any` used instead of `interface{}` in new code
