# Task 5 Report

## Implemented
- Added `backend/migrations/000001_initial_schema.up.sql` and `.down.sql` with the full schema from the brief, including `capacity_calculations`, neighborhood tables, watchlist tables, and indexes.
- Added `backend/internal/infrastructure/migrate/migrate.go` using `golang-migrate` with file-based migrations and `ErrNoChange` treated as success.
- Added GORM postgres support in `backend/internal/infrastructure/postgres/gorm/`:
  - `db.go`
  - `models.go`
  - `capacity_repository.go`
  - `capacity_repository_test.go`
- Wired `api` and `serve` modes to open PostgreSQL and use the GORM capacity repository.
- Wired `migrate up` and `migrate down` through the app entrypoint.
- Kept router/handler tests database-free by preserving injectable app seams and the in-memory router fallback.
- Switched default calculation IDs to UUIDs so they match the UUID primary key schema.

## Tested
- RED:
  - `go test ./internal/platform/app -run 'TestRun(MigrateUpRunsMigrations|StartsAPIModeWithInjectedCapacityApplication)' -count=1`
    - failed first on missing `runMigrations` / `openCapacityApplication` hooks.
  - `go test ./internal/infrastructure/postgres/gorm -run TestCapacityRepositoryPersistsAndFindsCalculations -count=1`
    - failed first because the migration/repository packages did not exist.
- GREEN:
  - `go test ./internal/platform/app -count=1`
  - `go test ./internal/infrastructure/postgres/gorm -run TestCapacityRepositoryPersistsAndFindsCalculations -count=1 -v`
    - `PROPULSE_TEST_DATABASE_URL is not set`, so the repo integration test skipped exactly as required.
  - `go test ./internal/infrastructure/... ./internal/interfaces/http/... -v`
  - `go test ./...`

## TDD Evidence
- RED failure output included:
  - `undefined: runMigrations`
  - `undefined: openCapacityApplication`
  - `no required module provides package github.com/propulse/propulse/backend/internal/infrastructure/migrate`
- GREEN output included:
  - `ok   github.com/propulse/propulse/backend/internal/platform/app`
  - `ok   github.com/propulse/propulse/backend/internal/infrastructure/postgres/gorm`
  - `SKIP: TestCapacityRepositoryPersistsAndFindsCalculations (PROPULSE_TEST_DATABASE_URL is not set)`

## Files Changed
- `backend/migrations/000001_initial_schema.up.sql`
- `backend/migrations/000001_initial_schema.down.sql`
- `backend/internal/infrastructure/migrate/migrate.go`
- `backend/internal/infrastructure/postgres/gorm/db.go`
- `backend/internal/infrastructure/postgres/gorm/models.go`
- `backend/internal/infrastructure/postgres/gorm/capacity_repository.go`
- `backend/internal/infrastructure/postgres/gorm/capacity_repository_test.go`
- `backend/internal/platform/app/app.go`
- `backend/internal/platform/app/app_test.go`
- `backend/internal/application/capacity/commands.go`
- `backend/go.mod`
- `backend/go.sum`

## Self-Review
- Repository persistence is now aligned with the app/application interfaces and the schema.
- HTTP unit tests still run without PostgreSQL.
- Migration runner is idempotent on `migrate up` when already current.

## Concerns
- The repository integration test was not exercised against a live PostgreSQL instance in this workspace because `PROPULSE_TEST_DATABASE_URL` was unset.
