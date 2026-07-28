# Boilerplate Repository Ports Refactor (Phase 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Decouple the service layer from the concrete Postgres implementation so alternate storage backends (Firestore, etc.) can be plugged in without editing services — the prerequisite for Phase 3.

**Architecture:** Introduce storage-neutral repository ports owned by the `repository` package. `repository.Repository` exposes interface fields (`Repository.User`, `Repository.Health`) instead of `Repository.Postgre.User`. The `pgsql` package becomes one adapter implementing those ports. Add a `UnitOfWork` port for multi-repository transactions and split `core.Setup` into dependency-building and HTTP-building halves so a future second binary can reuse the first. Also fixes the latent bug where `core.Teardown` never closes the DB (its `setDB` is never called).

**Tech Stack:** Go, GORM, go-sqlmock (existing repo tests), testify.

## Global Constraints

- Module path: `go-echo-boilerplate`.
- Test command: `go test -v -race ./...`; lint: `golangci-lint run ./...`.
- **No behavior change.** This is a pure refactor: every existing test must still pass, and HTTP responses must be byte-identical. New tests assert structural contracts only.
- The existing `pgsql` interfaces (`UserRepository`, `HealthRepository` in `internal/repository/pgsql/*.go`) already define the method sets; the ports mirror them exactly — do not change any method signature.
- Prereq ordering: this plan must land before Phase 3. It may land before or after Phase 1 (no overlap).

---

### Task 1: Define storage-neutral repository ports

Create the port interfaces in the `repository` package so services depend on `repository.UserRepository`, not `pgsql.UserRepository`.

**Files:**
- Create: `internal/repository/ports.go`
- Test: `internal/repository/ports_test.go`

**Interfaces:**
- Produces:
  - `repository.UserRepository` with methods `Create(ctx, *models.User) error`, `CheckByEmailOrPhoneNumber(ctx, email, phone string) (bool, error)`, `GetCredentialsByEmailOrPhoneNumber(ctx, email, phone string) (*models.User, error)`, `GetOneByAccountNumber(ctx, accountNumber string) (*models.User, error)`.
  - `repository.HealthRepository` with `Check(ctx) error`.

- [ ] **Step 1: Write the failing test (compile-time interface assertion)**

Create `internal/repository/ports_test.go`:

```go
package repository_test

import (
	"testing"

	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/repository/pgsql"

	"github.com/stretchr/testify/require"
)

// The pgsql adapters must satisfy the storage-neutral ports.
func TestPgsqlSatisfiesPorts(t *testing.T) {
	var _ repository.UserRepository = (pgsql.UserRepository)(nil)
	var _ repository.HealthRepository = (pgsql.HealthRepository)(nil)
	require.True(t, true)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/ -run TestPgsqlSatisfiesPorts -v`
Expected: FAIL to compile — `repository.UserRepository` / `repository.HealthRepository` undefined.

- [ ] **Step 3: Create `internal/repository/ports.go`**

```go
package repository

import (
	"context"
	"go-echo-boilerplate/internal/models"
)

// UserRepository is the storage-neutral port for user persistence. Adapters
// (pgsql, firestore, ...) implement it; services depend only on this interface.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	CheckByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (bool, error)
	GetCredentialsByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (*models.User, error)
	GetOneByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error)
}

// HealthRepository is the storage-neutral port for backend health checks.
type HealthRepository interface {
	Check(ctx context.Context) error
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/repository/ -run TestPgsqlSatisfiesPorts -v`
Expected: PASS — the pgsql interfaces structurally satisfy the ports (identical method sets).

- [ ] **Step 5: Commit**

```bash
git add internal/repository/ports.go internal/repository/ports_test.go
git commit -m "feat(repository): add storage-neutral repository ports"
```

---

### Task 2: Flatten the Repository aggregator to expose ports

Change `repository.Repository` from `{Postgre *pgsql.PostgreRepository}` to `{User UserRepository; Health HealthRepository}`, wiring the pgsql adapters in behind the ports.

**Files:**
- Modify: `internal/repository/main_repository.go`
- Test: `internal/repository/main_repository_test.go`

**Interfaces:**
- Consumes: `pgsql.New(db).User`, `pgsql.New(db).Health` (adapters from Task 1).
- Produces: `repository.Repository{User UserRepository; Health HealthRepository}`; `repository.New(*database.Database) *Repository` unchanged signature.

- [ ] **Step 1: Write the failing test**

Create `internal/repository/main_repository_test.go`:

```go
package repository_test

import (
	"testing"

	"go-echo-boilerplate/internal/repository"

	"github.com/stretchr/testify/require"
)

func TestRepositoryExposesPorts(t *testing.T) {
	// Zero-value struct: fields must be the port types, not a nested Postgre struct.
	var r repository.Repository
	require.Nil(t, r.User)
	require.Nil(t, r.Health)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/ -run TestRepositoryExposesPorts -v`
Expected: FAIL to compile — `r.User` / `r.Health` do not exist (only `r.Postgre`).

- [ ] **Step 3: Rewrite `internal/repository/main_repository.go`**

```go
package repository

import (
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/repository/pgsql"
)

type Repository struct {
	User   UserRepository
	Health HealthRepository
}

func New(database *database.Database) *Repository {
	postgre := pgsql.New(database.PostgreDatabase)
	return &Repository{
		User:   postgre.User,
		Health: postgre.Health,
	}
}
```

- [ ] **Step 4: Run test to verify it passes (repository package only)**

Run: `go test ./internal/repository/ -run TestRepositoryExposesPorts -v`
Expected: PASS. The service package will not compile yet — that is Task 3.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/main_repository.go internal/repository/main_repository_test.go
git commit -m "refactor(repository): expose ports instead of concrete Postgre struct"
```

---

### Task 3: Point services at the ports

Update all service call sites from `us.d.Repository.Postgre.User.X` to `us.d.Repository.User.X`.

**Files:**
- Modify: `internal/service/user_service.go` (lines calling `Repository.Postgre.User`)
- Modify: `internal/service/health_service.go` (calling `Repository.Postgre.Health`)
- Test: existing `internal/service/user_service_test.go`, `internal/service/health_service_test.go` (adjust mock wiring only)

**Interfaces:**
- Consumes: `repository.Repository{User, Health}` (Task 2).
- Produces: no new symbols; service behavior unchanged.

- [ ] **Step 1: Update the call sites**

In `internal/service/user_service.go`, replace every `us.d.Repository.Postgre.User` with `us.d.Repository.User` (3 occurrences: `Create`, `GetCredentialsByEmailOrPhoneNumber`, `GetOneByAccountNumber`). In `internal/service/health_service.go`, replace `Repository.Postgre.Health` with `Repository.Health`.

- [ ] **Step 2: Update the service tests' mock wiring**

In `internal/service/user_service_test.go` and `health_service_test.go`, wherever the mock repository is assembled into `repository.Repository{Postgre: ...}`, change it to set the port fields directly: `repository.Repository{User: mockUserRepo, Health: mockHealthRepo}`. The mock types already implement the port method sets (identical to the old pgsql interfaces).

- [ ] **Step 3: Run the service tests to verify they pass**

Run: `go test ./internal/service/ -v`
Expected: PASS — all pre-existing service tests green against the flattened repository.

- [ ] **Step 4: Verify the whole module builds**

Run: `go build ./...`
Expected: build succeeds — no remaining references to `Repository.Postgre`. Confirm with: `grep -rn "Repository.Postgre" internal` → no output.

- [ ] **Step 5: Commit**

```bash
git add internal/service/user_service.go internal/service/health_service.go internal/service/user_service_test.go internal/service/health_service_test.go
git commit -m "refactor(service): depend on repository ports, not pgsql"
```

---

### Task 4: UnitOfWork port for multi-repository transactions

Add a `WithinTransaction` port so services can compose writes across repositories atomically without importing GORM. Back it with the existing pgsql transaction repository.

**Files:**
- Modify: `internal/repository/ports.go` (add `UnitOfWork`)
- Modify: `internal/repository/main_repository.go` (wire it)
- Create: `internal/repository/pgsql/unit_of_work.go` (adapter)
- Test: `internal/repository/pgsql/unit_of_work_test.go`

**Interfaces:**
- Produces: `repository.UnitOfWork` with `WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error`; `repository.Repository` gains field `UnitOfWork UnitOfWork`.
- The transaction is carried on `ctx`; adapters read the ambient `*gorm.DB` (transactional or base) from context. Provide `pgsql.NewUnitOfWork(db *gorm.DB) repository.UnitOfWork`.

- [ ] **Step 1: Read the existing transaction repository**

Open `internal/repository/pgsql/transaction_pgsql_repository.go` to see the current transaction helper's shape and reuse its `db.Transaction(...)` pattern rather than inventing a new one.

- [ ] **Step 2: Write the failing test**

Create `internal/repository/pgsql/unit_of_work_test.go` using go-sqlmock (match the setup already used by other pgsql tests in this package):

```go
package pgsql_test

import (
	"context"
	"errors"
	"testing"

	"go-echo-boilerplate/internal/repository/pgsql"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return gdb, mock
}

func TestUnitOfWork_CommitOnSuccess(t *testing.T) {
	gdb, mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	uow := pgsql.NewUnitOfWork(gdb)
	err := uow.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_RollbackOnError(t *testing.T) {
	gdb, mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	uow := pgsql.NewUnitOfWork(gdb)
	err := uow.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return errors.New("boom")
	})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/repository/pgsql/ -run TestUnitOfWork -v`
Expected: FAIL to compile — `pgsql.NewUnitOfWork` undefined.

- [ ] **Step 4: Add the port**

In `internal/repository/ports.go`, add:

```go
// UnitOfWork runs a function inside a single storage transaction. The transaction
// handle is propagated via ctx so repositories enrolled in it commit or roll back
// together, without services depending on the storage driver.
type UnitOfWork interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
```

- [ ] **Step 5: Create `internal/repository/pgsql/unit_of_work.go`**

```go
package pgsql

import (
	"context"

	"go-echo-boilerplate/internal/repository"

	"gorm.io/gorm"
)

type txKey struct{}

// FromContext returns the transactional *gorm.DB stored on ctx, or the fallback
// base handle when the caller is not inside a WithinTransaction scope.
func FromContext(ctx context.Context, base *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return base
}

type unitOfWork struct {
	db *gorm.DB
}

// NewUnitOfWork builds the pgsql-backed UnitOfWork port.
func NewUnitOfWork(db *gorm.DB) repository.UnitOfWork {
	return &unitOfWork{db: db}
}

func (u *unitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}
```

- [ ] **Step 6: Wire it into the aggregator**

In `internal/repository/main_repository.go`, add the field and construction:

```go
type Repository struct {
	User       UserRepository
	Health     HealthRepository
	UnitOfWork UnitOfWork
}

func New(database *database.Database) *Repository {
	postgre := pgsql.New(database.PostgreDatabase)
	return &Repository{
		User:       postgre.User,
		Health:     postgre.Health,
		UnitOfWork: pgsql.NewUnitOfWork(database.PostgreDatabase),
	}
}
```

- [ ] **Step 7: Run tests and build**

Run: `go test ./internal/repository/... -v && go build ./...`
Expected: UnitOfWork tests PASS; build succeeds.

- [ ] **Step 8: Commit**

```bash
git add internal/repository/ports.go internal/repository/main_repository.go internal/repository/pgsql/unit_of_work.go internal/repository/pgsql/unit_of_work_test.go
git commit -m "feat(repository): add UnitOfWork port for cross-repo transactions"
```

---

### Task 5: Split core.Setup and fix DB teardown

`core.Setup` both builds dependencies and the HTTP server. Split it so a future `cmd/consumer` can reuse dependency wiring. Also fix the real bug: `core.Teardown` closes a `db` that `setDB` never populates, so the pool is never closed on shutdown.

**Files:**
- Modify: `internal/core/setup.go`
- Modify: `internal/core/teardown.go`
- Test: `internal/core/setup_test.go`

**Interfaces:**
- Produces:
  - `core.Dependencies{DB *database.Database; Service *service.Service; Config *config.Configuration; JWTConfig *jwtc.Configuration}`.
  - `core.BuildDependencies(config *config.Configuration) (*Dependencies, error)`.
  - `core.BuildHTTPServer(deps *Dependencies) *echo.Echo`.
  - `core.Setup(config)` retained as a thin wrapper: `BuildDependencies` then `BuildHTTPServer`, registering the DB for teardown.

- [ ] **Step 1: Write the failing test**

Create `internal/core/setup_test.go`:

```go
package core

import (
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/stretchr/testify/require"
)

// BuildDependencies must fail cleanly (not panic) when the DB cannot be reached,
// proving it is a distinct, testable step.
func TestBuildDependencies_BadDBReturnsError(t *testing.T) {
	cfg := &config.Configuration{}
	cfg.PostgreSQL.Host = "127.0.0.1"
	cfg.PostgreSQL.Port = 1 // nothing listening
	cfg.Application.Timezone = "UTC"

	_, err := BuildDependencies(cfg)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run TestBuildDependencies_BadDBReturnsError -v`
Expected: FAIL to compile — `BuildDependencies` undefined.

- [ ] **Step 3: Rewrite `internal/core/setup.go`**

```go
package core

import (
	"context"
	"go-echo-boilerplate/internal/config"
	handler "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/service"

	"github.com/labstack/echo/v4"
)

// Dependencies is the fully-wired application core, independent of any delivery
// mechanism. Both the HTTP server and future workers (e.g. a Kafka consumer)
// build from the same Dependencies.
type Dependencies struct {
	DB        *database.Database
	Service   *service.Service
	Config    *config.Configuration
	JWTConfig *jwtc.Configuration
}

// BuildDependencies initializes the logger, connects the database, and wires
// repositories and services. It performs no HTTP setup.
func BuildDependencies(configuration *config.Configuration) (*Dependencies, error) {
	logger.Initialize(configuration)

	db, err := database.Connect(configuration)
	if err != nil {
		logger.Instance.Error(context.Background(), "failed to connect to database", logger.Error(err))
		return nil, err
	}

	jwtConfig := jwtc.DefaultConfig(configuration)

	repo := repository.New(db)
	svc := service.New(service.Dependencies{
		Repository: *repo,
		Config:     configuration,
		JWTConfig:  jwtConfig,
	})

	return &Dependencies{
		DB:        db,
		Service:   svc,
		Config:    configuration,
		JWTConfig: jwtConfig,
	}, nil
}

// BuildHTTPServer wires the Echo server from already-built dependencies.
func BuildHTTPServer(deps *Dependencies) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	handler.New(e, deps.Service, deps.Config, deps.JWTConfig)
	return e
}

// Setup builds dependencies then the HTTP server, and registers the DB pool so
// Teardown can close it on shutdown.
func Setup(configuration *config.Configuration) (*echo.Echo, error) {
	deps, err := BuildDependencies(configuration)
	if err != nil {
		return nil, err
	}

	sqlDB, err := deps.DB.PostgreDatabase.DB()
	if err != nil {
		return nil, err
	}
	setDB(sqlDB)

	return BuildHTTPServer(deps), nil
}
```

- [ ] **Step 4: Confirm teardown now receives the pool**

`internal/core/teardown.go` already has `setDB`/`Teardown`; no change needed there — Setup now calls `setDB`, closing the previously-leaked gap. Leave `teardown.go` as is.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/core/ -run TestBuildDependencies_BadDBReturnsError -v`
Expected: PASS — connecting to a dead port returns an error rather than panicking.

- [ ] **Step 6: Build and run the full suite**

Run: `go build ./... && go test -race ./...`
Expected: build succeeds; all tests pass. `cmd/http/main.go` is unaffected (still calls `core.Setup`).

- [ ] **Step 7: Commit**

```bash
git add internal/core/setup.go internal/core/setup_test.go
git commit -m "refactor(core): split dependency and HTTP build; fix DB teardown leak"
```

---

## Final verification

- [ ] **No leftover concrete coupling + full suite**

```bash
grep -rn "Repository.Postgre" internal ; echo "^ expect no output"
go build ./... && go test -race ./... && golangci-lint run ./...
```

Expected: grep prints nothing; everything green.

## Self-Review notes

- **Spec coverage:** Phase 2 items 2.1–2.5 → Tasks 1 (ports defined/owned by service-side package), 2+3 (flatten + service call sites), 4 (UnitOfWork), 5 (split Setup). All covered. Bonus: Task 5 fixes the teardown DB-leak found during review.
- **Type consistency:** Port method sets copied verbatim from the existing `pgsql` interfaces (Task 1 constraint) so the compile-time assertion in Task 1 holds. `Repository{User, Health, UnitOfWork}` field names consistent across Tasks 2/4. `BuildDependencies`/`BuildHTTPServer`/`Dependencies` names consistent in Task 5.
- **Assumption to verify during execution:** Task 4 Step 1 — read `transaction_pgsql_repository.go` first; if it already exposes a compatible transaction helper, delegate to it instead of duplicating `db.Transaction`. Task 3 — confirm the exact mock type names in the existing service tests before rewiring.
