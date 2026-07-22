# Suggested Commands

## Daily Development
```bash
make dev              # Hot-reload via Air (also regenerates Swagger docs first)
make run-local        # Run with config/config.local.yaml (plain go run, JSON piped through jq)
make run-dev          # Run with config/config.dev.yaml
```

## Build
```bash
make build            # Dev build -> bin/
make build-prod       # Optimised Linux binary (CGO_ENABLED=0)
```

## Testing
```bash
make test             # go test -v -race ./...
make test-coverage    # With HTML coverage report in coverage/
go test ./internal/pkg/stringc/... -v   # Run a single package's tests
```

## Code Quality
```bash
make lint             # golangci-lint run ./...
make lint-fix         # golangci-lint run --fix ./...
go vet ./...          # Quick static analysis (no external tools needed)
make security-scan    # gosec
```

## Modules
```bash
make tidy             # go mod tidy && go mod verify
```

## Docs
```bash
make docs             # swag init -g cmd/http/main.go --output docs
```

## Database Migrations (Goose)
```bash
make migrate-up              # Apply all pending migrations
make migrate-down            # Roll back last migration
make migrate-status          # Show current migration state
make migrate-create NAME=create_sessions_table  # Create a new migration file
```

## Docker
```bash
make docker-up        # docker-compose up -d --build
make docker-down      # docker-compose down
make docker-logs      # Follow container logs
```

## Misc
```bash
make install-tools    # Install air, golangci-lint, swag, goose, gosec
make check-config     # Verify required config YAML files exist
make clean            # Remove bin/ and coverage/
```
