# Boilerplate Ops & Polish (Phase 4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining operational and correctness gaps — container image, CI pipeline, pagination conventions, bcrypt input guard, and repo hygiene — so the boilerplate is deployable and safe by default.

**Architecture:** Independent, mostly-additive changes: a multi-stage `Dockerfile`, a GitHub Actions workflow, a pagination helper in the response layer (the `PaginationOutput` struct already exists in `models/response_model.go`), a password-length guard in the validator, and a git-hygiene audit.

**Tech Stack:** Go, Docker, GitHub Actions, golangci-lint, gosec, testify.

## Global Constraints

- Module path: `go-echo-boilerplate`.
- Test command: `go test -v -race ./...`; lint: `golangci-lint run ./...`.
- No hard dependency on Phases 1–3; tasks here are standalone. Task 3 (pagination) pairs naturally with the existing `models.PaginationOutput` type.
- bcrypt's input limit is 72 bytes; inputs beyond that are silently truncated by the library.
- Binary output path matches the existing Makefile: `bin/server` from `./cmd/http`.

---

### Task 1: Password length guard (bcrypt 72-byte truncation)

bcrypt silently ignores bytes past 72, so `"<72 correct chars>X"` and `"<72 correct chars>Y"` verify identically. Reject over-long passwords at validation time.

**Files:**
- Create: `internal/pkg/validator/password_length.go`
- Modify: `internal/pkg/generator/hash.go` (guard `Hash` as defense in depth)
- Test: `internal/pkg/validator/password_length_test.go`

**Interfaces:**
- Produces: `validator.PasswordWithinBcryptLimit(password string) error` — returns an error when `len([]byte(password)) > 72`.
- `generator.Hash` returns an error for over-long input instead of hashing a truncated value.

- [ ] **Step 1: Write the failing test**

Create `internal/pkg/validator/password_length_test.go`:

```go
package validator_test

import (
	"strings"
	"testing"

	"go-echo-boilerplate/internal/pkg/validator"

	"github.com/stretchr/testify/require"
)

func TestPasswordWithinBcryptLimit(t *testing.T) {
	require.NoError(t, validator.PasswordWithinBcryptLimit(strings.Repeat("a", 72)))
	require.Error(t, validator.PasswordWithinBcryptLimit(strings.Repeat("a", 73)))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pkg/validator/ -run TestPasswordWithinBcryptLimit -v`
Expected: FAIL to compile — `PasswordWithinBcryptLimit` undefined.

- [ ] **Step 3: Create the validator**

`internal/pkg/validator/password_length.go`:

```go
package validator

import "fmt"

// bcryptMaxBytes is bcrypt's hard input limit; bytes beyond it are silently
// truncated, so two different long passwords could hash identically.
const bcryptMaxBytes = 72

// PasswordWithinBcryptLimit rejects passwords longer than bcrypt can safely hash.
func PasswordWithinBcryptLimit(password string) error {
	if len([]byte(password)) > bcryptMaxBytes {
		return fmt.Errorf("password must not exceed %d bytes", bcryptMaxBytes)
	}
	return nil
}
```

- [ ] **Step 4: Guard `generator.Hash` as well**

In `internal/pkg/generator/hash.go`, at the top of `Hash`, add:

```go
	if len([]byte(password)) > 72 {
		return "", fmt.Errorf("password exceeds bcrypt's 72-byte limit")
	}
```

(`fmt` is already imported.)

- [ ] **Step 5: Run tests to verify pass**

Run: `go test ./internal/pkg/validator/ ./internal/pkg/generator/ -v`
Expected: PASS.

- [ ] **Step 6: Enforce in the create-user path**

In `internal/service/user_service.go` `Create`, before hashing, add:

```go
	if err := validator.PasswordWithinBcryptLimit(request.Password); err != nil {
		return nil, errorc.Error(errorc.ErrorInvalidInput, err, "Password too long")
	}
```

Run: `go build ./...` — expected success.

- [ ] **Step 7: Commit**

```bash
git add internal/pkg/validator/password_length.go internal/pkg/validator/password_length_test.go internal/pkg/generator/hash.go internal/service/user_service.go
git commit -m "fix(auth): reject passwords over bcrypt's 72-byte limit"
```

---

### Task 2: Pagination request parsing + response helper

`models.PaginationOutput` exists but nothing parses page params or builds it. Add a request parser and a success-list-with-pagination response helper so downstream projects have a convention.

**Files:**
- Create: `internal/models/pagination_model.go` (request input + defaults)
- Create: `internal/pkg/response/pagination_response.go`
- Test: `internal/models/pagination_model_test.go`

**Interfaces:**
- Produces:
  - `models.PaginationInput{Limit int; Offset int}` with `models.ParsePagination(limitStr, offsetStr string) models.PaginationInput` (defaults: limit 20, max 100, offset 0).
  - `response.SuccessListPaginated(ctx echo.Context, status int, message string, data any, pagination *models.PaginationOutput) error`.

- [ ] **Step 1: Write the failing test**

Create `internal/models/pagination_model_test.go`:

```go
package models_test

import (
	"testing"

	"go-echo-boilerplate/internal/models"

	"github.com/stretchr/testify/require"
)

func TestParsePagination_Defaults(t *testing.T) {
	p := models.ParsePagination("", "")
	require.Equal(t, 20, p.Limit)
	require.Equal(t, 0, p.Offset)
}

func TestParsePagination_CapsLimit(t *testing.T) {
	p := models.ParsePagination("500", "40")
	require.Equal(t, 100, p.Limit)
	require.Equal(t, 40, p.Offset)
}

func TestParsePagination_IgnoresGarbage(t *testing.T) {
	p := models.ParsePagination("abc", "-5")
	require.Equal(t, 20, p.Limit)
	require.Equal(t, 0, p.Offset)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/models/ -run TestParsePagination -v`
Expected: FAIL to compile — `ParsePagination` undefined.

- [ ] **Step 3: Create the model**

`internal/models/pagination_model.go`:

```go
package models

import "strconv"

const (
	defaultLimit = 20
	maxLimit     = 100
)

// PaginationInput is the normalized, validated pagination request.
type PaginationInput struct {
	Limit  int
	Offset int
}

// ParsePagination converts raw query strings into a safe PaginationInput,
// applying defaults and an upper bound so clients cannot request unbounded pages.
func ParsePagination(limitStr, offsetStr string) PaginationInput {
	limit := defaultLimit
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		limit = v
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := 0
	if v, err := strconv.Atoi(offsetStr); err == nil && v > 0 {
		offset = v
	}

	return PaginationInput{Limit: limit, Offset: offset}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/models/ -run TestParsePagination -v`
Expected: PASS.

- [ ] **Step 5: Add the response helper**

Read `internal/pkg/response/success_response.go` to match the existing `Success`/`SuccessList` construction style, then create `internal/pkg/response/pagination_response.go`:

```go
package response

import (
	"go-echo-boilerplate/internal/models"

	"github.com/labstack/echo/v4"
)

// SuccessListPaginated writes a list response with a pagination block, following
// the same envelope shape as SuccessList.
func SuccessListPaginated(ctx echo.Context, status int, message string, data any, pagination *models.PaginationOutput) error {
	return ctx.JSON(status, models.Response{
		Code:       status,
		Status:     "OK",
		Message:    message,
		Data:       data,
		Pagination: pagination,
	})
}
```

Confirm the field names (`Code`, `Status`, `Message`, `Data`, `Pagination`) match `models.Response` — they do per `response_model.go`. Match how the existing helpers set `Metadata`/`Status` (copy that detail if `SuccessList` populates them).

- [ ] **Step 6: Build**

Run: `go build ./... && go test ./internal/models/ ./internal/pkg/response/ -v`
Expected: build succeeds; tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/models/pagination_model.go internal/models/pagination_model_test.go internal/pkg/response/pagination_response.go
git commit -m "feat(api): pagination request parsing and paginated list response"
```

---

### Task 3: Multi-stage Dockerfile

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`

- [ ] **Step 1: Create `.dockerignore`**

```
bin/
tmp/
coverage/
*.log
.env
.git
.serena
docs/superpowers
```

- [ ] **Step 2: Create `Dockerfile`**

```dockerfile
# --- build stage ---
FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/http

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/server /app/server
COPY config/ /app/config/
COPY api-docs.html /app/api-docs.html
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/server"]
```

Note: confirm the Go version line matches `go.mod`'s `go` directive; adjust `golang:1.XX-alpine` accordingly. If `cmd/consumer` exists (Phase 3), optionally add a second build line `RUN ... go build -o /out/consumer ./cmd/consumer` and a separate image or build arg.

- [ ] **Step 3: Verify the image builds**

Run: `docker build -t go-echo-boilerplate:local .`
Expected: image builds; final stage runs as non-root `appuser`.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "chore(docker): multi-stage build image for the HTTP server"
```

---

### Task 4: CI pipeline (lint, test, gosec)

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create the workflow**

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [master]

jobs:
  build-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Build
        run: go build ./...
      - name: Test
        run: go test -race ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: gosec
        uses: securego/gosec@master
        with:
          args: ./...
```

- [ ] **Step 2: Validate YAML locally**

Run: `go run gopkg.in/yaml.v3 2>/dev/null; python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); print('ok')"`
Expected: prints `ok` (YAML parses). If Python is unavailable, visually confirm indentation.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: build, test, lint, and gosec on PRs"
```

---

### Task 5: Repo hygiene audit

Verify no secrets are tracked and that generated/log artifacts are ignored.

**Files:**
- Modify (if needed): `.gitignore`

- [ ] **Step 1: Confirm sensitive files are untracked**

Run:

```bash
git ls-files | grep -E '\.env$|server\.log$|security-report\.json$|config\.(dev|prod|uat|local)\.yaml$' || echo "none tracked"
```

Expected: `none tracked`. If any appear, remove from the index: `git rm --cached <file>` and ensure the pattern is in `.gitignore`.

- [ ] **Step 2: Scan history for a committed .env or secrets**

Run:

```bash
git log --all --diff-filter=A --name-only --pretty=format: | sort -u | grep -E '\.env$|credentials|serviceAccount' || echo "no secret files ever added"
```

Expected: `no secret files ever added`. If a secret file was ever committed, rotate the leaked credentials and note that history rewrite (e.g. `git filter-repo`) is required — do not attempt the rewrite as part of this task; open a separate ticket.

- [ ] **Step 3: Ensure config example is the only tracked config**

Run: `git ls-files config/`
Expected: only `config/config.local.example.yaml` (and any intentionally-committed non-secret env config). Confirm real per-env files stay ignored.

- [ ] **Step 4: Commit any .gitignore changes**

```bash
git add .gitignore
git commit -m "chore: harden gitignore for secrets and artifacts" || echo "nothing to commit"
```

---

## Final verification

- [ ] **Full suite + lint + build + image**

```bash
go build ./... && go test -race ./... && golangci-lint run ./... && docker build -t go-echo-boilerplate:local .
```

Expected: all green; image builds.

## Self-Review notes

- **Spec coverage:** Phase 4 items 4.1–4.5 → Task 3 (Dockerfile), Task 4 (CI+gosec), Task 2 (pagination), Task 1 (bcrypt guard), Task 5 (repo hygiene). All covered.
- **Type consistency:** `models.ParsePagination`/`PaginationInput` and `response.SuccessListPaginated` referencing the existing `models.PaginationOutput`/`models.Response` fields — names verified against `response_model.go`.
- **Assumptions to verify during execution:** (1) Dockerfile Go base image tag must match `go.mod`'s `go` directive — read it first. (2) Task 2 Step 5 — match the exact envelope-population style of the existing `SuccessList` (Metadata/Status), read `success_response.go` before writing. (3) gosec action may flag pre-existing findings; triage and either fix or annotate rather than blanket-suppress.
```
