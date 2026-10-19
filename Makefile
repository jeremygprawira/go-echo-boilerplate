.PHONY: help ci pr-base hooks unhooks build build-prod run dev clean test test-coverage test-integration docker-up docker-down docker-logs docker-clean docs docs-check lint lint-fix tidy migrate-up migrate-down migrate-status migrate-create install-tools security-scan

# Default target
.DEFAULT_GOAL := help

BINARY_NAME=server
MAIN_FILE=cmd/http/main.go
DOCKER_COMPOSE_FILE=docker-compose.yml
DOCKER_COMPOSE = $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo docker-compose)

# Default environment (can be overridden: make run ENV=dev)
ENV ?= dev

# Database connection string for migrations
# Override with: make migrate-up DB_DSN="postgres://..."
DB_DSN ?= postgres://$${DB_USER}:$${DB_PASSWORD}@$${DB_HOST}:$${DB_PORT}/$${DB_NAME}?sslmode=$${DB_SSL_MODE}

GO_TOOL := go tool -modfile=tools/go.mod

# Build flags for production
BUILD_FLAGS=-ldflags="-s -w"

help:
	@echo "🏗️  Build & Development:"
	@echo "  make build                    - Build the application binary"
	@echo "  make build-prod               - Build a stripped static Linux binary"
	@echo "  make run                      - Run with config (ENV=local|dev|uat|prod)"
	@echo "  make dev                      - Run with hot-reload (air)"
	@echo "  make clean                    - Remove binaries and artifacts"
	@echo "  make tidy                     - Clean and download Go modules"
	@echo ""
	@echo "📦 Container & Deploy:"
	@echo "  make docker-up                - Start the app and Postgres (needs config/config.dev.yaml)"
	@echo "  make docker-down              - Stop services"
	@echo "  make docker-logs              - View container logs"
	@echo "  make docker-clean             - Remove containers, volumes, and images"
	@echo ""
	@echo "🗄️  Database & Migrations:"
	@echo "  make migrate-up               - Apply migrations to the database that .env's DB_* values name"
	@echo "  make migrate-down             - Rollback migrations"
	@echo "  make migrate-status           - Check migration status"
	@echo "  make migrate-create NAME=xxx  - Create new migration file"
	@echo ""
	@echo "🧪 Testing & Quality:"
	@echo "  make ci                       - Run every check CI runs"
	@echo "  make pr-base PR=<number>      - Check a PR's base branch (needs gh)"
	@echo "  make hooks                    - Run make ci before every git push"
	@echo "  make unhooks                  - Turn the pre-push hook off"
	@echo "  make test                     - Run all tests"
	@echo "  make test-coverage            - Run tests with coverage report"
	@echo "  make test-integration         - Run the integration suite on a throwaway Postgres (needs Docker)"
	@echo "  make lint                     - Run linter (golangci-lint)"
	@echo "  make lint-fix                 - Run linter and auto-fix issues"
	@echo "  make security-scan            - Run security vulnerability scan"
	@echo ""
	@echo "📚 Documentation & Tools:"
	@echo "  make docs                     - Generate Swagger documentation"
	@echo "  make docs-check               - Fail if the committed Swagger docs are stale"
	@echo "  make install-tools            - Install development tools"
	@echo ""
	@echo "💡 Examples:"
	@echo "  make run ENV=dev              - Run with dev config"
	@echo "  make test-coverage            - Get test coverage HTML report"
	@echo ""

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

run:
	@echo "🚀 Running application with ENV=$(ENV)..."
	@if [ ! -f config/config.$(ENV).yaml ]; then \
		echo "❌ ERROR: config/config.$(ENV).yaml not found"; \
		exit 1; \
	fi
	@ENV=$(ENV) go run $(MAIN_FILE) | jq -R '. as $$line | try (fromjson) catch $$line'

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
	@scripts/test-integration.sh

ci:
	go mod tidy -diff
	cd tools && go mod tidy -diff
	go build ./...
	go test -race ./...
	$(GO_TOOL) golangci-lint run ./...
	$(GO_TOOL) gosec -quiet ./...
	scripts/check-repo.sh
	scripts/check-deadcode.sh
	@$(MAKE) --no-print-directory docs-check

docs-check:
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
		$(GO_TOOL) swag init --quiet -g $(MAIN_FILE) --output "$$tmp/docs" && \
		for f in docs.go swagger.json swagger.yaml; do \
			diff -u "docs/$$f" "$$tmp/docs/$$f" || { echo "❌ docs/$$f is stale. Run make docs"; exit 1; }; \
		done

pr-base:
	@if [ -z "$(PR)" ]; then \
		echo "❌ ERROR: Please specify PR=<number>"; \
		echo "Example: make pr-base PR=14"; \
		exit 1; \
	fi
	scripts/check-pr-base.sh "$(PR)"

hooks:
	git config core.hooksPath .githooks
	@echo "✅ git push now runs make ci first. Skip once with git push --no-verify"

unhooks:
	git config --unset core.hooksPath || true
	@echo "✅ Pre-push hook off"

lint:
	@echo "🔍 Running linter..."
	@$(GO_TOOL) golangci-lint run ./...

lint-fix:
	@echo "🔧 Running linter with auto-fix..."
	@$(GO_TOOL) golangci-lint run --fix ./...

security-scan:
	@echo "🔒 Running security scan..."
	@$(GO_TOOL) gosec -fmt=json -out=security-report.json ./... || true
	@$(GO_TOOL) gosec ./...

docs:
	@echo "📚 Generating Swagger documentation..."
	@$(GO_TOOL) swag init -g $(MAIN_FILE) --output docs
	@echo "✅ Swagger docs generated in ./docs"

docker-up:
	@if [ ! -f config/config.dev.yaml ]; then \
		echo "❌ ERROR: config/config.dev.yaml not found. Copy config/config.local.example.yaml to it, set environment: dev, and set the postgresql host to pgdb, port 5432"; \
		if [ -d config/config.dev.yaml ]; then echo "   It is a directory, which Docker makes for a missing file. Remove it with: rmdir config/config.dev.yaml"; fi; \
		exit 1; \
	fi
	@echo "🐳 Starting containers..."
	@$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) up -d --build
	@echo "✅ Containers started. On a new database, run make migrate-up"
	@$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) ps

docker-down:
	@echo "🐳 Stopping containers..."
	@$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) down
	@echo "✅ Containers stopped"

docker-logs:
	@$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) logs -f --tail=100

docker-clean:
	@echo "🧹 Cleaning Docker resources..."
	@$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) down -v --remove-orphans
	@docker system prune -f
	@echo "✅ Docker cleanup complete"

migrate-up:
	@echo "📊 Running migrations on the database that .env names..."
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

install-tools:
	@echo "🔧 Installing development tools..."
	@echo "Installing air (hot-reload)..."
	@go install github.com/air-verse/air@latest
	@echo "Installing goose (migrations)..."
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "✅ All tools installed successfully"
