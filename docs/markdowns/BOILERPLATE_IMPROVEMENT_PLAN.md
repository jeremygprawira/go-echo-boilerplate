# Boilerplate Improvement Plan

Companion to [BOILERPLATE_ASSESSMENT.md](./BOILERPLATE_ASSESSMENT.md). Ordered by risk-to-effort: security bugs first (small, self-contained), then the refactor that unlocks pluggable backends, then the backends themselves, then polish.

Legend — effort: S (<½ day), M (1–2 days), L (3+ days). Each phase is independently shippable.

---

## Phase 0 — Critical security fixes (do first, ship as one PR)

Small, self-contained, no architecture change. Highest risk reduction.

| # | Task | Effort | Assessment ref |
|---|------|--------|----------------|
| 0.1 | Fix JWT validation secret. Split `validator.JWT` into access/refresh paths, each verifying with the matching secret. `AccessToken()` → `AccessTokenSecret`, `RefreshToken()` → `RefreshTokenSecret`. Add a test that fails when the two secrets differ. | S | Sec #1 |
| 0.2 | Fix `config.local.example.yaml` key `bearer:` → `access:`. Add fail-fast config validation at startup: non-empty secrets, distinct access/refresh secrets, non-empty API key, valid durations. Fail `core.Setup` if invalid. | S | Sec #2 |
| 0.3 | Remove hardcoded default-secret fallbacks in `generator/jwt.go`. Return error when config nil/empty. | S | Sec #3 |
| 0.4 | CORS from config. Replace `AllowOrigins: ["*"]` with `config.CORS.AllowedOrigins` per env. Never `*` + `AllowCredentials: true`. | S | Sec #4 |
| 0.5 | Constant-time API key compare (`crypto/subtle.ConstantTimeCompare`). Confirm `X-API-Key` masked in logger. | S | Sec #7 |
| 0.6 | Login enumeration fix. Single generic "invalid credentials" error for both not-found and bad-password. Dummy bcrypt compare on not-found path to flatten timing. | S | Sec #6 |

**Exit:** `make test` + `make lint` green. New tests: JWT secret split, config validation rejects empty/duplicate secrets, login returns identical error for both failure modes.

---

## Phase 1 — Baseline HTTP hardening (one PR)

| # | Task | Effort | Assessment ref |
|---|------|--------|----------------|
| 1.1 | Rate limiting middleware on `/users` (register) and `/users/tokens` (login). Echo `middleware.RateLimiter` or Redis-backed once Phase 3 lands. | M | Sec #8 |
| 1.2 | Request body size limit (`middleware.BodyLimit`). | S | Sec #8 |
| 1.3 | Security headers (`middleware.Secure()`). | S | Sec #8 |
| 1.4 | Per-request timeout middleware wired to `Application.Timeout`; propagate `ctx` deadline into service/repo calls. | M | Sec #8, Gaps |
| 1.5 | Gate Swagger UI behind non-prod env check. | S | Sec #8 |
| 1.6 | Env-var override support: `viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))` + explicit `BindEnv` for secrets. Verify `POSTGRESQL_PASSWORD` overrides YAML. | S | Sec #9 |
| 1.7 | Request ID middleware + propagate into wide-event log. | S | Gaps |

**Exit:** manual verify each middleware fires; env-var override test; Swagger 404 in prod.

---

## Phase 2 — Repository ports refactor (unlocks everything below)

This is the keystone. Do before adding any backend.

| # | Task | Effort | Assessment ref |
|---|------|--------|----------------|
| 2.1 | Define repo interfaces owned by service layer: `UserRepository`, `HealthRepository`. Move interface defs out of `pgsql`. | M | Flex |
| 2.2 | Flatten `repository.Repository` — expose `Repository.User` (interface), not `Repository.Postgre.User` (concrete). Update all service call sites (`user_service.go:102` etc). | M | Flex |
| 2.3 | `pgsql` package becomes one adapter implementing the interfaces. No behavior change. Existing sqlmock tests still pass. | S | Flex |
| 2.4 | `UnitOfWork` / `WithinTransaction(ctx, fn)` port so services compose multi-repo writes without touching GORM. Wrap existing `transaction_pgsql_repository.go`. | M | Gaps |
| 2.5 | Split `core.Setup` into `BuildDependencies()` (config→clients→repos→services) and `BuildHTTPServer(deps)`. Enables a second binary later. | S | Flex |

**Exit:** services reference only interfaces; `grep -r "Postgre.User" internal/service` empty; all tests green; no functional change.

---

## Phase 3 — Infrastructure clients layer

New `internal/clients/`, one package per backend. Same pattern PostgreSQL uses: config section → `Connect` → registered in `Clients` struct → cleanup in `core/teardown.go`. Each section optional via `enabled:` flag; app runs with Postgres alone.

| # | Task | Effort | Assessment ref |
|---|------|--------|----------------|
| 3.1 | `internal/clients/` scaffold + `Clients` aggregator + enabled-flag config gating + health checks iterate enabled clients. | M | Flex |
| 3.2 | **Redis** (`internal/clients/redis`): connect + `Cache` interface (cache-aside in adapter layer, not services) + token denylist/session store keyed by JTI. | M | Flex, Sec #5 |
| 3.3 | **Refresh + revocation flow** on top of 3.2: `/tokens/refresh` endpoint, logout (denylist JTI), reject denylisted tokens in `BearerAuthMiddleware`. Add JTI claim to tokens. | M | Sec #5 |
| 3.4 | **Kafka** (`internal/clients/kafka`): `Publisher` port (swap NATS/RabbitMQ/PubSub later) + consumer loops via `graceful.AddProcess`. New `cmd/consumer/main.go` reusing `BuildDependencies()`. Document outbox pattern for post-DB-write events. | L | Flex |
| 3.5 | **Firebase** (`internal/clients/firebase`): one Firebase app → Firestore, FCM, Firebase Auth. Firestore plugs in as a `UserRepository` adapter behind Phase 2 interfaces. | L | Flex |

**Exit:** boilerplate runs Postgres-only with all flags off; each backend connects when enabled; health check reports each; Firestore adapter passes same repo interface tests as pgsql.

---

## Phase 4 — Ops & polish

| # | Task | Effort | Assessment ref |
|---|------|--------|----------------|
| 4.1 | Dockerfile (multi-stage build → `bin/server`). | S | Gaps |
| 4.2 | CI pipeline: `make lint` + `make test` + `gosec` on PR. | M | Gaps |
| 4.3 | Pagination/list conventions in response envelope (`models/response_model.go`) — set the pattern for downstream projects. | M | Gaps |
| 4.4 | Reject/pre-hash passwords >72 bytes in validator (bcrypt truncation). | S | Gaps |
| 4.5 | Repo hygiene: confirm `server.log`, `security-report.json`, `.env` never committed; scan git history for leaked secrets. | S | Gaps |

---

## Sequencing summary

```
Phase 0 (sec bugs)  ──► Phase 1 (HTTP hardening)
                             │
Phase 2 (repo ports) ◄───────┘   [keystone — must precede Phase 3]
        │
        ├──► Phase 3.1 clients scaffold
        │        ├──► 3.2 Redis ──► 3.3 refresh/revocation
        │        ├──► 3.4 Kafka
        │        └──► 3.5 Firebase/Firestore
        │
        └──► Phase 4 (ops & polish, parallelizable)
```

Phase 0 ships today. Phase 2 is the gate — no backend work is worth starting until services depend on interfaces, or every backend re-does the coupling fix.
