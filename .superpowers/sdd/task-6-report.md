## What I Implemented

- Added pure neighborhood signal domain rules in `backend/internal/domain/neighborhood`, porting `evaluateNeighborhoodSignal` behavior for the required scenarios:
  - 青枫花园 => `适合砍价`, `high`
  - 云澜府 => `价格偏硬`, `low`
- Added neighborhood application ports and service methods for:
  - creating/getting neighborhoods
  - adding/listing watchlist items
  - retrieving latest metric snapshots with evaluated signal output
- Added GORM models/repository support for existing Task 5 tables:
  - `neighborhoods`
  - `watchlist_items`
  - `neighborhood_metrics`
- Added idempotent repository behavior:
  - natural-key lookup before creating neighborhoods
  - `ON CONFLICT DO NOTHING` for `(user_id, neighborhood_id)` watchlist rows
  - demo seed skips metric insertion when metrics already exist for a seeded neighborhood
- Added HTTP handlers and route wiring for:
  - `POST /api/v1/neighborhoods`
  - `GET /api/v1/neighborhoods/:id`
  - `GET /api/v1/neighborhoods/:id/metrics`
  - `POST /api/v1/watchlist/items`
  - `GET /api/v1/watchlist`
- Added `demo-user` as the temporary watchlist user ID.
- Added `PROPULSE_SEED_DEMO_DATA=true` config and startup seeding for:
  - 青枫花园 / 滨江核心 / 三房
  - 云澜府 / 城东新区 / 四房
- Updated `backend/api/openapi.yaml` with the new endpoint and schema definitions.

## What I Tested And Results

- `cd backend && go test ./internal/domain/neighborhood ./internal/application/neighborhood ./internal/interfaces/http/... -v`
  - Result: PASS
- `cd backend && go test ./...`
  - Result: PASS
- `cd backend && PROPULSE_SEED_DEMO_DATA=true go run ./cmd/propulse api`
  - Result: could not complete in this environment because the default local PostgreSQL role `propulse` does not exist.
  - Relevant output: `FATAL: role "propulse" does not exist (SQLSTATE 28000)`

## TDD Evidence

### RED

Domain/application first run:

```text
$ cd backend && go test ./internal/domain/neighborhood ./internal/application/neighborhood -v
github.com/propulse/propulse/backend/internal/domain/neighborhood: no non-test Go files
internal/domain/neighborhood/signal_test.go:6:12: undefined: EvaluateSignal
internal/domain/neighborhood/signal_test.go:6:27: undefined: SignalInput
FAIL
```

HTTP handler first run:

```text
$ cd backend && go test ./internal/interfaces/http/handler -v
internal/interfaces/http/handler/neighborhood_test.go:30:39: undefined: NewNeighborhood
internal/interfaces/http/handler/neighborhood_test.go:165:41: undefined: NewWatchlist
FAIL
```

Router first run:

```text
$ cd backend && go test ./internal/interfaces/http/router -v
internal/interfaces/http/router/router_test.go:155:3: unknown field NeighborhoodApplication in struct literal of type Dependencies
FAIL
```

GORM repository first run:

```text
$ cd backend && go test ./internal/infrastructure/postgres/gorm -v
internal/infrastructure/postgres/gorm/neighborhood_repository_test.go:31:10: undefined: NewNeighborhoodRepository
internal/infrastructure/postgres/gorm/neighborhood_repository_test.go:46:40: undefined: NeighborhoodMetricModel
FAIL
```

Config seed flag first run:

```text
$ cd backend && go test ./internal/infrastructure/config -v
internal/infrastructure/config/config_test.go:33:9: cfg.SeedDemoData undefined
internal/infrastructure/config/config_test.go:46:10: cfg.SeedDemoData undefined
FAIL
```

App wiring first run:

```text
$ cd backend && go test ./internal/platform/app -v
internal/platform/app/app_test.go:77:41: undefined: openNeighborhoodApplication
internal/platform/app/app_test.go:103:92: undefined: NeighborhoodApplication
FAIL
```

### GREEN

Focused task verification:

```text
$ cd backend && go test ./internal/domain/neighborhood ./internal/application/neighborhood ./internal/interfaces/http/... -v
PASS
ok github.com/propulse/propulse/backend/internal/domain/neighborhood
ok github.com/propulse/propulse/backend/internal/application/neighborhood
ok github.com/propulse/propulse/backend/internal/interfaces/http/handler
ok github.com/propulse/propulse/backend/internal/interfaces/http/router
```

Full backend verification:

```text
$ cd backend && go test ./...
ok github.com/propulse/propulse/backend/internal/application/capacity
ok github.com/propulse/propulse/backend/internal/application/neighborhood
ok github.com/propulse/propulse/backend/internal/domain/capacity
ok github.com/propulse/propulse/backend/internal/domain/neighborhood
ok github.com/propulse/propulse/backend/internal/infrastructure/config
ok github.com/propulse/propulse/backend/internal/infrastructure/postgres/gorm
ok github.com/propulse/propulse/backend/internal/interfaces/http/handler
ok github.com/propulse/propulse/backend/internal/interfaces/http/router
ok github.com/propulse/propulse/backend/internal/platform/app
ok github.com/propulse/propulse/backend/web
```

## Files Changed

- `backend/api/openapi.yaml`
- `backend/internal/domain/neighborhood/signal.go`
- `backend/internal/domain/neighborhood/signal_test.go`
- `backend/internal/application/neighborhood/ports.go`
- `backend/internal/application/neighborhood/commands.go`
- `backend/internal/application/neighborhood/queries.go`
- `backend/internal/application/neighborhood/service_test.go`
- `backend/internal/infrastructure/config/config.go`
- `backend/internal/infrastructure/config/config_test.go`
- `backend/internal/infrastructure/postgres/gorm/models.go`
- `backend/internal/infrastructure/postgres/gorm/neighborhood_repository.go`
- `backend/internal/infrastructure/postgres/gorm/neighborhood_repository_test.go`
- `backend/internal/interfaces/http/handler/capacity_test.go`
- `backend/internal/interfaces/http/handler/neighborhood.go`
- `backend/internal/interfaces/http/handler/neighborhood_test.go`
- `backend/internal/interfaces/http/handler/watchlist.go`
- `backend/internal/interfaces/http/router/neighborhood_memory.go`
- `backend/internal/interfaces/http/router/router.go`
- `backend/internal/interfaces/http/router/router_test.go`
- `backend/internal/platform/app/app.go`
- `backend/internal/platform/app/app_test.go`

## Self-Review Findings

- Domain package does not import Gin/GORM/sqlc/asynq/redis/chromedp.
- Application package depends on repository interfaces, not concrete database code.
- HTTP handlers only adapt protocol/request/response behavior and map application errors.
- No Task 7 import endpoint, Task 8 metric calculation job, or frontend changes were implemented.
- `listedHomesChangePct` is retained in application/domain input but is not present in the Task 5 database schema. Repository-backed records therefore default it to `0`; seeded data still produces the required statuses using the available metric fields.

## Concerns

- Live seed/curl verification could not be completed because the local default PostgreSQL role `propulse` does not exist and no `PROPULSE_TEST_DATABASE_URL` was set in the environment.
