.PHONY: help build build-prod build-consumer run run-consumer dev clean test test-coverage test-integration docker-up docker-down docker-logs docker-clean docs lint lint-fix tidy migrate-up migrate-down migrate-status migrate-create check-config install-tools security-scan run-local run-dev run-uat run-prod

# Default target
.DEFAULT_GOAL := help

# For detailed documentation, see docs/MAKEFILE_GUIDE.md

# ============================================================================
# Configuration Variables
# ============================================================================
BINARY_NAME=server
MAIN_FILE=cmd/http/main.go
DOCKER_COMPOSE_FILE=docker-compose.yml

# Environment-specific config files
CONFIG_LOCAL=config/config.local.yaml
CONFIG_DEV=config/config.dev.yaml
CONFIG_UAT=config/config.uat.yaml
CONFIG_PROD=config/config.prod.yaml

# Default environment (can be overridden: make run ENV=dev)
ENV ?= dev

# Database connection string for migrations
# Override with: make migrate-up DB_DSN="postgres://..."
DB_DSN ?= postgres://$${DB_USER}:$${DB_PASSWORD}@$${DB_HOST}:$${DB_PORT}/$${DB_NAME}?sslmode=$${DB_SSL_MODE}

# Build flags for production
BUILD_FLAGS=-ldflags="-s -w -X main.Version=$$(git describe --tags --always --dirty) -X main.BuildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# ============================================================================
# Help & Documentation
# ============================================================================
help:
	@echo "╔════════════════════════════════════════════════════════════════╗"
	@echo "║          GO-ECHO-BOILERPLATE - Makefile Commands               ║"
	@echo "║          See docs/MAKEFILE_GUIDE.md for details                ║"
	@echo "╚════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "🏗️  Build & Development:"
	@echo "  make build                    - Build the application binary"
	@echo "  make build-prod               - Build optimized production binary"
	@echo "  make build-consumer           - Build the Kafka consumer binary"
	@echo "  make run                      - Run with config (ENV=local|dev|uat|prod)"
	@echo "  make run-consumer             - Run the Kafka consumer (ENV=local|dev|uat|prod)"
	@echo "  make dev                      - Run with hot-reload (air)"
	@echo "  make clean                    - Remove binaries and artifacts"
	@echo "  make tidy                     - Clean and download Go modules"
	@echo ""
	@echo "🌍 Environment-Specific Runs:"
	@echo "  make run-local                - Run with config.local.yaml"
	@echo "  make run-dev                  - Run with config.dev.yaml"
	@echo "  make run-uat                  - Run with config.uat.yaml"
	@echo "  make run-prod                 - Run with config.prod.yaml"
	@echo ""
	@echo "📦 Container & Deploy:"
	@echo "  make docker-up                - Start services with Docker Compose"
	@echo "  make docker-down              - Stop services"
	@echo "  make docker-logs              - View container logs"
	@echo "  make docker-clean             - Remove containers, volumes, and images"
	@echo ""
	@echo "🗄️  Database & Migrations:"
	@echo "  make migrate-up               - Run migrations (ENV=local)"
	@echo "  make migrate-down             - Rollback migrations"
	@echo "  make migrate-status           - Check migration status"
	@echo "  make migrate-create NAME=xxx  - Create new migration file"
	@echo ""
	@echo "🧪 Testing & Quality:"
	@echo "  make test                     - Run all tests"
	@echo "  make test-coverage            - Run tests with coverage report"
	@echo "  make test-integration         - Run integration tests"
	@echo "  make lint                     - Run linter (golangci-lint)"
	@echo "  make lint-fix                 - Run linter and auto-fix issues"
	@echo "  make security-scan            - Run security vulnerability scan"
	@echo ""
	@echo "📚 Documentation & Tools:"
	@echo "  make docs                     - Generate Swagger documentation"
	@echo "  make install-tools            - Install development tools"
	@echo "  make check-config             - Validate config files"
	@echo "  make check-port               - Check port availability"
	@echo ""
	@echo "💡 Examples:"
	@echo "  make run ENV=dev              - Run with dev config"
	@echo "  make migrate-up ENV=uat       - Run UAT migrations"
	@echo "  make test-coverage            - Get test coverage HTML report"
	@echo ""
	@echo "🔗 Detailed Guide: docs/MAKEFILE_GUIDE.md"
	@echo ""

# ============================================================================
# Configuration Validation
# ============================================================================
check-config:
	@echo "🔍 Checking configuration files..."
	@if [ ! -f $(CONFIG_LOCAL) ]; then echo "⚠️  $(CONFIG_LOCAL) not found"; fi
	@if [ -f $(CONFIG_LOCAL) ]; then echo "✅ $(CONFIG_LOCAL) exists"; fi
	@if [ -f $(CONFIG_DEV) ]; then echo "✅ $(CONFIG_DEV) exists"; fi
	@if [ -f $(CONFIG_UAT) ]; then echo "✅ $(CONFIG_UAT) exists"; fi
	@if [ -f $(CONFIG_PROD) ]; then echo "✅ $(CONFIG_PROD) exists"; fi
	@echo ""

# ============================================================================
# Build Commands
# ============================================================================
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p bin
	@go build -o bin/$(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Build complete: bin/$(BINARY_NAME)"

build-prod:
	@echo "🔨 Building production binary with optimizations..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/$(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Production build complete: bin/$(BINARY_NAME)"
	@ls -lh bin/$(BINARY_NAME)

build-consumer:
	@echo "🔨 Building consumer binary..."
	@mkdir -p bin
	@go build -o bin/consumer ./cmd/consumer
	@echo "✅ Build complete: bin/consumer"

# ============================================================================
# Run Commands (Environment-Specific)
# ============================================================================
run:
	@echo "🚀 Running application with ENV=$(ENV)..."
	@if [ ! -f config/config.$(ENV).yaml ]; then \
		echo "❌ ERROR: config/config.$(ENV).yaml not found"; \
		exit 1; \
	fi
	@ENV=$(ENV) go run $(MAIN_FILE) | jq -R '. as $$line | try (fromjson) catch $$line'

run-local:
	@$(MAKE) run ENV=local

run-dev:
	@$(MAKE) run ENV=dev

run-uat:
	@$(MAKE) run ENV=uat

run-prod:
	@$(MAKE) run ENV=prod

run-consumer:
	@echo "🚀 Running consumer with ENV=$(ENV)..."
	@ENV=$(ENV) go run ./cmd/consumer

# Hot-reload development (requires air: go install github.com/air-verse/air@latest)
dev:
	@echo "Create or Update swagger documentation"
	@$(MAKE) docs
	@echo "🔥 Starting hot-reload development server..."
	@if ! command -v air > /dev/null; then \
		echo "⚠️  'air' not found. Run 'make install-tools'"; \
		exit 1; \
	fi
	@ENV=${ENV} air

# ============================================================================
# Testing Commands
# ============================================================================
test:
	@echo "🧪 Running tests..."
	@go test -v -race ./...

test-coverage:
	@echo "🧪 Running tests with coverage..."
	@mkdir -p coverage
	@go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "✅ Coverage report generated: coverage/coverage.html"
	@go tool cover -func=coverage/coverage.out | grep total

test-integration:
	@echo "🧪 Running integration tests..."
	@go test -v -race -tags=integration ./tests/...

# ============================================================================
# Code Quality
# ============================================================================
lint:
	@echo "🔍 Running linter..."
	@if ! command -v golangci-lint > /dev/null; then \
		echo "⚠️  golangci-lint not installed. Run 'make install-tools'"; \
		exit 1; \
	fi
	@golangci-lint run ./...

lint-fix:
	@echo "🔧 Running linter with auto-fix..."
	@golangci-lint run --fix ./...

security-scan:
	@echo "🔒 Running security scan..."
	@if ! command -v gosec > /dev/null; then \
		echo "⚠️  Installing gosec..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi
	@gosec -fmt=json -out=security-report.json ./... || true
	@gosec ./...

# ============================================================================
# Documentation
# ============================================================================
docs:
	@echo "📚 Generating Swagger documentation..."
	@if ! command -v swag > /dev/null; then \
		echo "⚠️  swag not installed. Installing..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	fi
	@swag init -g $(MAIN_FILE) --output docs
	@echo "✅ Swagger docs generated in ./docs"

# ============================================================================
# Docker Commands
# ============================================================================
docker-up:
	@echo "🐳 Starting containers..."
	@docker-compose -f $(DOCKER_COMPOSE_FILE) up -d --build
	@echo "✅ Containers started"
	@docker-compose ps

docker-down:
	@echo "🐳 Stopping containers..."
	@docker-compose -f $(DOCKER_COMPOSE_FILE) down
	@echo "✅ Containers stopped"

docker-logs:
	@docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f --tail=100

docker-clean:
	@echo "🧹 Cleaning Docker resources..."
	@docker-compose -f $(DOCKER_COMPOSE_FILE) down -v --remove-orphans
	@docker system prune -f
	@echo "✅ Docker cleanup complete"

# ============================================================================
# Database Migrations
# ============================================================================
migrate-up:
	@echo "📊 Running migrations (ENV=$(ENV))..."
	@if ! command -v goose > /dev/null; then \
		echo "⚠️  goose not installed. Run 'make install-tools'"; \
		exit 1; \
	fi
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	goose -dir migration/db/postgre postgres "$(DB_DSN)" up
	@echo "✅ Migrations applied"

migrate-down:
	@echo "📊 Rolling back migrations..."
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	goose -dir migration/db/postgre postgres "$(DB_DSN)" down
	@echo "✅ Migration rolled back"

migrate-status:
	@echo "📊 Checking migration status..."
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	goose -dir migration/db/postgre postgres "$(DB_DSN)" status

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "❌ ERROR: Please specify NAME=<migration_name>"; \
		echo "Example: make migrate-create NAME=create_users_table"; \
		exit 1; \
	fi
	@if ! command -v goose > /dev/null; then \
		echo "⚠️  goose not installed. Run 'make install-tools'"; \
		exit 1; \
	fi
	@goose -dir migration/db/postgre create $(NAME) sql
	@echo "✅ Migration created in migration/db/postgre"

# ============================================================================
# Maintenance
# ============================================================================
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/
	@rm -rf coverage/
	@rm -f security-report.json
	@echo "✅ Cleanup complete"

tidy:
	@echo "📦 Tidying modules..."
	@go mod tidy
	@go mod verify
	@echo "✅ Modules tidied"

# ============================================================================
# Tool Installation
# ============================================================================
install-tools:
	@echo "🔧 Installing development tools..."
	@echo "Installing air (hot-reload)..."
	@go install github.com/air-verse/air@latest
	@echo "Installing golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Installing swag (Swagger)..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@echo "Installing goose (migrations)..."
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "Installing gosec (security)..."
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "✅ All tools installed successfully"

check-port:
	lsof -i :8080