# Makefile Quick Reference

## 🚀 Most Used Commands

```bash
# Development
make dev                    # Hot-reload development (recommended)
make run-local             # Run with local config
make run ENV=dev           # Run with specific environment

# Testing
make test                  # Run all tests
make test-coverage         # Tests with HTML coverage report
make lint                  # Check code quality

# Database
make migrate-up            # Apply migrations
make migrate-create NAME=add_users  # Create new migration

# Docker
make docker-up             # Start containers
make docker-down           # Stop containers
make docker-logs           # View logs

# Production
make build-prod            # Build optimized binary
make security-scan         # Security check
```

## 📋 Environment Variables

```bash
ENV=local|dev|uat|prod     # Select environment config
DB_DSN="postgres://..."    # Override database connection
```

## 🛠️ First Time Setup

```bash
make install-tools         # Install all dev tools
make check-config          # Verify config files exist
make docker-up             # Start database
make migrate-up            # Run migrations
make dev                   # Start development
```

## 📊 Pre-Commit Checklist

```bash
make test-coverage         # ✓ Tests pass with good coverage
make lint                  # ✓ Code quality checks
make security-scan         # ✓ No vulnerabilities
make docs                  # ✓ Update API docs
```

## 🔧 Troubleshooting

| Issue           | Solution                            |
| --------------- | ----------------------------------- |
| Tool not found  | `make install-tools`                |
| Config missing  | `make check-config`                 |
| Migration fails | Check `DB_DSN` and `make docker-up` |
| Build fails     | `make clean && make tidy`           |

---

**Full documentation**: See [MAKEFILE_GUIDE.md](./MAKEFILE_GUIDE.md)
