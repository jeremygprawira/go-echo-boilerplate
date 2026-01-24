# Makefile Commands Reference

This document provides detailed information about the enhanced Makefile commands for the go-echo-boilerplate project.

## Quick Start

```bash
# View all available commands
make help

# Install all development tools
make install-tools

# Run with hot-reload for development
make dev

# Run with specific environment
make run ENV=dev
```

## Environment-Specific Development

The Makefile supports multiple environments through YAML configuration files:

- **local**: `config/config.local.yaml` (for local development)
- **dev**: `config/config.dev.yaml` (for development environment)
- **uat**: `config/config.uat.yaml` (for UAT/staging)
- **prod**: `config/config.prod.yaml` (for production)

### Running with Different Environments

```bash
# Run with local config (default)
make run-local

# Run with dev config
make run-dev

# Run with UAT config
make run-uat

# Run with production config
make run-prod

# Or use the ENV variable
make run ENV=uat
```

## Development Workflow

### Hot-Reload Development

For the best development experience, use `air` for hot-reload:

```bash
# Start development server with hot-reload
make dev
```

This will:

- Automatically install `air` if not present
- Watch for file changes in `.go`, `.yaml`, `.yml` files
- Rebuild and restart the server on changes
- Use `config.local.yaml` by default

### Building

```bash
# Build development binary
make build

# Build optimized production binary (with version info)
make build-prod
```

The production build includes:

- Optimized binary size (`-s -w` flags)
- Git version tag
- Build timestamp
- Cross-compilation for Linux AMD64

## Testing & Quality Assurance

### Running Tests

```bash
# Run all tests with race detection
make test

# Run tests with coverage report (generates HTML)
make test-coverage

# Run integration tests
make test-integration
```

### Code Quality

```bash
# Run linter
make lint

# Run linter and auto-fix issues
make lint-fix

# Run security vulnerability scan
make security-scan
```

## Database Migrations

### Running Migrations

```bash
# Run migrations (automatically loads variables from .env if present)
make migrate-up

# Run migrations for specific environment
make migrate-up ENV=dev

# Rollback last migration
make migrate-down

# Check migration status
make migrate-status

# Create new migration
make migrate-create NAME=create_users_table
```

### Database Connection String

Override the default database connection:

```bash
make migrate-up DB_DSN="postgres://user:pass@host:5432/dbname?sslmode=disable"
```

## Docker Operations

```bash
# Start all containers
make docker-up

# Stop containers
make docker-down

# View logs (tail 100 lines, follow)
make docker-logs

# Clean up containers, volumes, and images
make docker-clean
```

## Documentation

```bash
# Generate Swagger documentation
make docs

# Check which config files exist
make check-config
```

## Tool Installation

Install all required development tools at once:

```bash
make install-tools
```

This installs:

- **air**: Hot-reload for Go applications
- **golangci-lint**: Fast Go linters runner
- **swag**: Swagger documentation generator
- **goose**: Database migration tool
- **gosec**: Security scanner for Go code

## Maintenance

```bash
# Clean build artifacts
make clean

# Tidy and verify Go modules
make tidy
```

## Production Deployment

### Building for Production

```bash
# Build optimized binary
make build-prod

# The binary will be in bin/server
# It includes version info from git tags
```

### Running in Production

```bash
# Run with production config
make run-prod

# Or use Docker
make docker-up
```

## Advanced Usage Examples

### Multi-Environment Testing

```bash
# Test locally
make run-local

# Test with dev config
make run-dev

# Test with UAT config
make run-uat
```

### Complete Development Cycle

```bash
# 1. Install tools (first time only)
make install-tools

# 2. Check configs
make check-config

# 3. Start database
make docker-up

# 4. Run migrations
make migrate-up

# 5. Start development with hot-reload
make dev
```

### Pre-Deployment Checklist

```bash
# 1. Run tests with coverage
make test-coverage

# 2. Run linter
make lint

# 3. Run security scan
make security-scan

# 4. Build production binary
make build-prod

# 5. Generate docs
make docs
```

## Configuration Files

### Required Files

- `config/config.local.yaml` - Local development
- `config/config.dev.yaml` - Development environment
- `config/config.uat.yaml` - UAT/Staging environment
- `config/config.prod.yaml` - Production environment

### Creating Config Files

Copy the example and customize:

```bash
cp config/config.local.example.yaml config/config.local.yaml
# Edit config/config.local.yaml with your settings
```

## Troubleshooting

### Tool Not Found

If you get "command not found" errors:

```bash
make install-tools
```

### Config File Missing

```bash
# Check which configs exist
make check-config

# Create missing config from example
cp config/config.local.example.yaml config/config.local.yaml
```

### Migration Errors

Ensure your database is running and `DB_DSN` is correct:

```bash
# Start database
make docker-up

# Check migration status
make migrate-status
```

## Tips & Best Practices

1. **Use `make dev` for development** - It provides the best DX with hot-reload
2. **Run `make test-coverage` before commits** - Ensure code quality
3. **Use environment-specific configs** - Never hardcode credentials
4. **Run `make security-scan` regularly** - Catch vulnerabilities early
5. **Use `make build-prod` for deployments** - Get optimized binaries with version info
6. **Check `make help` when in doubt** - Always up-to-date command reference
