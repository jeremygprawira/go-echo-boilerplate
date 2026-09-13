# Boilerplate Improvement Plan — Status

Companion to [BOILERPLATE_ASSESSMENT.md](./BOILERPLATE_ASSESSMENT.md). This was originally a phased plan (security fixes → repo-ports refactor → pluggable Redis/Kafka/Firebase → ops polish). Phases 0–2 and 4 shipped; Phase 3 (infra clients) was implemented and then deliberately removed — the boilerplate stays Postgres-only.

## Shipped

- **Phase 0 — security fixes**: JWT secret split, config key fix, no hardcoded secret fallback, CORS from config, constant-time API key compare, login-enumeration fix.
- **Phase 1 — HTTP hardening**: rate limiting, body size limit, security headers, per-request timeout, Swagger gated off in prod.
- **Phase 2 — repository ports refactor**: services depend on `repository.UserRepository`/`HealthRepository` interfaces; `core.Setup` is split into `BuildDependencies()` + `BuildHTTPServer(deps)`.
- **Phase 4 — ops & polish**: Dockerfile + CI (`make lint`/`make test`/gosec), pagination convention in the response envelope, bcrypt 72-byte truncation handled.

## Reverted

- **Phase 3 — infrastructure clients (Redis/Kafka/Firebase)**: all three were built (Redis cache + JTI revocation store, Kafka publisher/consumer, Firebase/Firestore `UserRepository` adapter), then removed by decision. The refresh/revocation *flow* from 3.3 stayed — `/tokens/refresh` and `/logout` still exist — but `tokenstore.TokenStore` now only has `NewNoopStore()`, so revocation doesn't actually take effect (see JWT_USAGE.md). Re-adding a real backend means implementing `TokenStore` (and, if needed, a `Cache` port and a `UserRepository` adapter) again, not reopening this phase's old code.

## Still open

- **Env-var config overrides** (originally Phase 1.6): `viper.AutomaticEnv()` has no `SetEnvKeyReplacer`/`BindEnv`, so nested keys like `postgresql.password` can't be overridden via env vars. Only relevant if/when this boilerplate needs container-native secret injection instead of YAML files on disk.
