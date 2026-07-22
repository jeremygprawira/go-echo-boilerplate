# Code Style & Conventions

## General
- **Go 1.24**, standard `gofmt` formatting assumed.
- Use `any` instead of `interface{}` (Go 1.18+).
- All exported functions and types must have godoc comments; internal helpers are optional but encouraged.
- Error messages in godoc examples use the `//` indented example style (Go doc format).

## Naming
- Packages are short, lowercase, sometimes abbreviated: `errorc`, `stringc`, `boolc`, `numberc`, `jwtc`.
- Interfaces are named after the capability: `UserService`, `UserRepository`.
- Implementations are unexported structs: `userService`, `userRepository`.
- Constructors are `NewXxx(deps) Interface`.
- Constants that wrap PostgreSQL error codes: `pgUniqueViolation = "23505"` (defined at package level with a comment linking to PG docs).

## Error Handling
- **Use `errorc.Error(predefined, internalErr, "optional message format %s", args...)`** — never return raw errors directly to handlers.
- Predefined sentinel errors live in `internal/pkg/errorc/` (e.g. `errorc.ErrorDatabase`, `errorc.ErrorAlreadyExist`).
- `HTTPError.Error()` returns the **safe public message only** — never the raw internal error string.
- `HTTPError.Internal()` returns the raw error — used only by the logging layer.
- `response.Error(ctx, err)` is the **single choke-point** for converting errors to HTTP JSON responses AND logging them.
- Do NOT call `logger.AddError` from service or repository layers — let `response.Error` handle it centrally.

## Logging (Wide-Event Pattern)
- One structured log line per request, emitted by the logger middleware at response time.
- **Never scatter `fmt.Println` or raw zap calls** in business logic.
- Enrich the wide event with `logger.AddToKey(ctx, "groupKey", ...)` for grouped structured fields.
- All request-related business data for a domain should be grouped under a named key (e.g. `"user"`, `"event"`).
- Use a single `logger.AddToKey` call at the top of each service method with all known fields upfront — do not call `logger.Add` and `logger.AddToKey` on the same key (fragile).
- `logger.AddTo` and `logger.MergeTo` are **deprecated** — use `logger.AddToKey`.

## HTTP Layer
- All handlers call `response.Error(ctx, err)` for errors and `response.Success(ctx, data)` for success.
- Validation errors use `response.ErrorValidation(ctx, validationErrors)`.
- Handler functions return `error` (Echo convention).

## Database
- GORM with `PreferSimpleProtocol: true`.
- GORM logger: `Silent` in `production`, `Info` in all other environments (local, dev, uat).
- DSN timezone sourced from `config.Application.Timezone` — never hardcoded.
- PostgreSQL unique-violation detection: `errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation` (avoid pre-insert existence checks — rely on DB constraints).

## Config
- Environment-specific YAML files: `config/config.{local,dev,uat,prod}.yaml`.
- Loaded via Viper. `ENV` env var selects the file.
- Key environment values: `application.environment` = `"local"`, `"dev"`, `"uat"`, `"production"`.

## Testing
- Table-driven tests with `[]struct{ input, expected }` pattern.
- Use `github.com/stretchr/testify` for assertions.
- Race detector always enabled: `go test -race ./...`.
