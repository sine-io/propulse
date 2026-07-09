# Propulse Go Integrated Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first production-shaped Propulse backend phase: one Go binary serves the exported Next frontend, REST APIs, PostgreSQL persistence, Redis/Asynq jobs, manual import, and one neighborhood metric calculation loop.

**Architecture:** Keep the existing React/Next frontend at the repository root and add a Go backend under `backend/`. The Go service follows DDD Lite + CQRS + Clean Architecture + DIP: domain code is pure, application handlers depend on ports, infrastructure implements those ports, and HTTP/worker layers adapt protocols.

**Tech Stack:** Go, Gin, PostgreSQL, Redis/Asynq, GORM, sqlc, golang-migrate, zerolog, OpenAPI, Next static export, `go:embed`.

## Global Constraints

- 后端使用 Go。
- 前端 React 保持不变，但嵌入 Go 后端。
- 一个 `go build` 产出一个可启动完整服务的二进制。
- 同时支持 Docker Compose 启动。
- 日志使用 `zerolog`。
- 架构遵循 DDD Lite + CQRS + Clean Architecture + DIP。
- 第一版按完整产品后端设计，包括自动采集、人工导入/修正和未来 API 数据源扩展。
- 前端不能依赖 Next SSR。
- 不使用 Next Server Actions。
- 不使用 Next API Routes 承担后端逻辑。
- 动态数据统一通过 Go REST API 获取。
- HTTP routes: `/` embedded frontend, `/api/v1/*` product API, `/admin/api/*` admin API, `/healthz`, `/readyz`, optional `/metrics`.
- One binary modes: `serve`, `api`, `worker`, `scheduler`, `migrate up`, `migrate down`.

---

## File Structure

- Create `backend/go.mod`: isolated Go module for the backend binary.
- Create `backend/cmd/propulse/main.go`: CLI entrypoint for all modes.
- Create `backend/internal/platform/app`: process composition, mode wiring, graceful shutdown.
- Create `backend/internal/infrastructure/config`: environment parsing and defaults.
- Create `backend/internal/infrastructure/logger`: zerolog setup.
- Create `backend/internal/interfaces/http`: Gin engine, middleware, route registration, DTO mapping.
- Create `backend/web`: embedded static frontend filesystem and copied static assets.
- Create `backend/internal/domain/capacity`: pure housing capacity rules ported from `src/lib/decision.ts`.
- Create `backend/internal/domain/neighborhood`: pure neighborhood signal rules.
- Create `backend/internal/domain/decision`: pure action-window recommendation rules.
- Create `backend/internal/application/capacity`: command/query handlers for persisted capacity calculations.
- Create `backend/internal/application/neighborhood`: command/query handlers for watchlist and metrics.
- Create `backend/internal/application/collection`: command handlers for manual imports and source records.
- Create `backend/internal/application/metric`: command handler for neighborhood metric calculation.
- Create `backend/internal/application/queue`: queue port definitions.
- Create `backend/internal/infrastructure/postgres/gorm`: GORM models and CRUD repositories.
- Create `backend/internal/infrastructure/postgres/sqlc`: sqlc generated package.
- Create `backend/internal/infrastructure/queue`: Asynq client/server wiring and task handlers.
- Create `backend/internal/infrastructure/migrate`: migration runner.
- Create `backend/migrations`: SQL migrations.
- Create `backend/queries`: sqlc query files.
- Create `backend/api/openapi.yaml`: REST contract used by frontend.
- Create `backend/web/static/.gitkeep`: destination for exported frontend assets.
- Create `scripts/sync-static-web.mjs`: copies `out/` into `backend/web/static/`.
- Modify `next.config.mjs`: enable static export.
- Modify `package.json`: add static build, sync, client generation, and full verification scripts.
- Modify `Dockerfile`: build Next static output, build Go binary, ship one runtime image.
- Modify `docker-compose.yml`: run app, worker, scheduler, postgres, redis from the same image.
- Create `src/lib/api-client.ts`: generated-client wrapper and fetch fallback behavior.
- Modify `src/components/calculator-panel.tsx`: submit to Go API and render persisted result.
- Modify `src/components/watchlist-page.tsx`: load watchlist from Go API.
- Modify `src/components/action-window-page.tsx`: load action-window recommendation from Go API.

---

### Task 1: Go Module, CLI Modes, Config, And Logger

**Files:**
- Create: `backend/go.mod`
- Create: `backend/cmd/propulse/main.go`
- Create: `backend/internal/infrastructure/config/config.go`
- Create: `backend/internal/infrastructure/config/config_test.go`
- Create: `backend/internal/infrastructure/logger/logger.go`
- Create: `backend/internal/platform/app/app.go`
- Create: `backend/internal/platform/app/app_test.go`

**Interfaces:**
- Produces: `config.Load() (config.Config, error)`
- Produces: `logger.New(config.LogConfig) zerolog.Logger`
- Produces: `app.Run(ctx context.Context, mode string, cfg config.Config, log zerolog.Logger) error`
- Consumes: no earlier task output.

- [ ] **Step 1: Create failing config tests**

```go
package config

import "testing"

func TestLoadUsesDocumentedDefaults(t *testing.T) {
	t.Setenv("PROPULSE_HTTP_ADDR", "")
	t.Setenv("PROPULSE_DATABASE_URL", "")
	t.Setenv("PROPULSE_REDIS_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL must have a local postgres default")
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("RedisAddr = %q, want 127.0.0.1:6379", cfg.RedisAddr)
	}
	if cfg.Log.Level != "info" {
		t.Fatalf("Log.Level = %q, want info", cfg.Log.Level)
	}
}
```

- [ ] **Step 2: Run config test to verify it fails**

Run: `cd backend && go test ./internal/infrastructure/config -run TestLoadUsesDocumentedDefaults -v`

Expected: FAIL because `Load` and `Config` are not defined.

- [ ] **Step 3: Implement config and logger**

Use these exact config fields:

```go
type Config struct {
	HTTPAddr    string
	DatabaseURL string
	RedisAddr   string
	Mode        string
	Log         LogConfig
}

type LogConfig struct {
	Level string
	Pretty bool
}
```

Environment variables:

```text
PROPULSE_HTTP_ADDR
PROPULSE_DATABASE_URL
PROPULSE_REDIS_ADDR
PROPULSE_LOG_LEVEL
PROPULSE_LOG_PRETTY
```

Local defaults:

```text
HTTPAddr=:8080
DatabaseURL=postgres://propulse:propulse@127.0.0.1:5432/propulse?sslmode=disable
RedisAddr=127.0.0.1:6379
Log.Level=info
Log.Pretty=false
```

- [ ] **Step 4: Add CLI mode dispatch**

`backend/cmd/propulse/main.go` must accept:

```text
propulse serve
propulse api
propulse worker
propulse scheduler
propulse migrate up
propulse migrate down
```

Unknown modes must exit non-zero and print:

```text
usage: propulse [serve|api|worker|scheduler|migrate up|migrate down]
```

- [ ] **Step 5: Add app mode tests**

```go
func TestNormalizeModeAcceptsDocumentedModes(t *testing.T) {
	for _, args := range [][]string{
		{"serve"},
		{"api"},
		{"worker"},
		{"scheduler"},
		{"migrate", "up"},
		{"migrate", "down"},
	} {
		mode, err := NormalizeMode(args)
		if err != nil {
			t.Fatalf("NormalizeMode(%v) error = %v", args, err)
		}
		if mode == "" {
			t.Fatalf("NormalizeMode(%v) returned empty mode", args)
		}
	}
}
```

- [ ] **Step 6: Verify task**

Run:

```bash
cd backend
go mod tidy
go test ./...
go build -o bin/propulse ./cmd/propulse
./bin/propulse --help
```

Expected:

```text
go test ./... PASS
go build exits 0
help output includes all documented modes
```

- [ ] **Step 7: Commit**

```bash
git add backend/go.mod backend/go.sum backend/cmd backend/internal/infrastructure/config backend/internal/infrastructure/logger backend/internal/platform/app
git commit -m "feat: add go backend process skeleton"
```

---

### Task 2: Static Next Export And Embedded Frontend Server

**Files:**
- Modify: `next.config.mjs`
- Modify: `package.json`
- Create: `scripts/sync-static-web.mjs`
- Create: `backend/web/web.go`
- Create: `backend/web/web_test.go`
- Create: `backend/web/static/.gitkeep`

**Interfaces:**
- Consumes: `app.Run(...)` from Task 1.
- Produces: `web.FS() http.FileSystem`
- Produces: package script `build:web` that leaves assets under `backend/web/static/`.

- [ ] **Step 1: Write failing embed test**

```go
package web

import (
	"io/fs"
	"testing"
)

func TestEmbeddedWebContainsIndex(t *testing.T) {
	embedded := Embedded()
	if _, err := fs.Stat(embedded, "index.html"); err != nil {
		t.Fatalf("index.html missing from embedded web fs: %v", err)
	}
}
```

Expected initial failure: `Embedded` is undefined.

- [ ] **Step 2: Enable static export**

Change `next.config.mjs` to:

```js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "export",
  reactStrictMode: true,
};

export default nextConfig;
```

- [ ] **Step 3: Add static sync script**

`scripts/sync-static-web.mjs` must:

```js
import { cp, mkdir, rm } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const source = resolve(root, "out");
const target = resolve(root, "backend", "web", "static");

await rm(target, { recursive: true, force: true });
await mkdir(target, { recursive: true });
await cp(source, target, { recursive: true });
```

- [ ] **Step 4: Add package scripts**

Add:

```json
"build:web": "pnpm build && node scripts/sync-static-web.mjs",
"verify": "pnpm typecheck && pnpm lint && pnpm test"
```

Keep existing `build`, `lint`, `test`, and `typecheck` scripts.

- [ ] **Step 5: Implement embedded filesystem**

`backend/web/web.go` must use package name `web` and embed the `static` directory:

```go
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embedded embed.FS

func Embedded() fs.FS
```

`Embedded()` must return `fs.Sub(embedded, "static")` and panic only if the embedded subdirectory is missing at build time.

- [ ] **Step 6: Verify task**

Run:

```bash
pnpm build:web
cd backend
go test ./web -run TestEmbeddedWebContainsIndex -v
go test ./...
```

Expected:

```text
backend/web/static/index.html exists
go tests PASS
```

- [ ] **Step 7: Commit**

```bash
git add next.config.mjs package.json pnpm-lock.yaml scripts/sync-static-web.mjs backend/web
git commit -m "feat: embed static frontend in go backend"
```

---

### Task 3: Gin Router, Health Checks, Static Fallback, And Request Logging

**Files:**
- Create: `backend/internal/interfaces/http/router/router.go`
- Create: `backend/internal/interfaces/http/router/router_test.go`
- Create: `backend/internal/interfaces/http/middleware/request_id.go`
- Create: `backend/internal/interfaces/http/middleware/logging.go`
- Modify: `backend/internal/platform/app/app.go`

**Interfaces:**
- Consumes: `web.Embedded() fs.FS`
- Produces: `router.New(deps router.Dependencies) *gin.Engine`
- Produces: health routes `/healthz`, `/readyz`
- Produces: static frontend fallback for `/`, `/calculator`, `/watchlist`, `/action-window`, `/neighborhoods`, `/methods`, `/templates`.

- [ ] **Step 1: Write failing router tests**

```go
func TestHealthAndReadyRoutes(t *testing.T) {
	engine := New(Dependencies{})

	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rec.Code)
		}
	}
}

func TestAPI404DoesNotReturnFrontend(t *testing.T) {
	engine := New(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Implement middleware**

`request_id.go` must:

```go
const HeaderRequestID = "X-Request-Id"
```

For each request:

```text
Use inbound X-Request-Id if present.
Otherwise generate a UUID.
Set X-Request-Id on the response.
Store request_id in Gin context.
```

`logging.go` must emit one JSON log event with:

```text
request_id
method
path
status
latency_ms
```

- [ ] **Step 3: Implement route groups**

Route shape:

```go
api := engine.Group("/api/v1")
admin := engine.Group("/admin/api")
```

At this task, both groups may only expose a JSON 404 for unknown routes. Do not map API 404s to the frontend.

- [ ] **Step 4: Wire `api` and `serve` mode**

`api` starts HTTP only. `serve` starts HTTP and later tasks will add scheduler and worker.

- [ ] **Step 5: Verify task**

Run:

```bash
cd backend
go test ./internal/interfaces/http/... ./internal/platform/app/... -v
go build -o bin/propulse ./cmd/propulse
PROPULSE_HTTP_ADDR=:18080 ./bin/propulse api
```

In another terminal:

```bash
curl -i http://127.0.0.1:18080/healthz
curl -i http://127.0.0.1:18080/calculator
curl -i http://127.0.0.1:18080/api/v1/missing
```

Expected:

```text
/healthz returns 200
/calculator returns frontend HTML
/api/v1/missing returns 404 JSON
request logs are JSON and include request_id
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/interfaces/http backend/internal/platform/app
git commit -m "feat: serve api health checks and embedded frontend"
```

---

### Task 4: Capacity Domain And REST API

**Files:**
- Create: `backend/internal/domain/capacity/capacity.go`
- Create: `backend/internal/domain/capacity/capacity_test.go`
- Create: `backend/internal/application/capacity/commands.go`
- Create: `backend/internal/application/capacity/queries.go`
- Create: `backend/internal/application/capacity/ports.go`
- Create: `backend/internal/application/capacity/service_test.go`
- Create: `backend/internal/interfaces/http/handler/capacity.go`
- Create: `backend/internal/interfaces/http/handler/capacity_test.go`
- Modify: `backend/internal/interfaces/http/router/router.go`
- Create: `backend/api/openapi.yaml`

**Interfaces:**
- Consumes: `router.New(...)`
- Produces: `capacity.Calculate(input capacity.HousingCapacityInput) capacity.HousingCapacityResult`
- Produces: application port `CalculationRepository`
- Produces: `POST /api/v1/capacity/calculations`
- Produces: `GET /api/v1/capacity/calculations/:id`

- [ ] **Step 1: Port existing TypeScript rule tests into Go**

Use the same scenario from `src/lib/decision.test.ts`:

```go
func TestCalculateHousingCapacityClassifiesStrained(t *testing.T) {
	result := Calculate(HousingCapacityInput{
		CashOnHand:                 150,
		OldHomeValue:               320,
		OldLoanBalance:             80,
		MonthlyIncome:              3.5,
		CurrentMonthlyMortgage:     0,
		AcceptableMonthlyMortgage:  1.5,
		TargetTotalPrice:           550,
		RenovationBudget:           40,
		TransactionCosts:           18,
		TransitionRentCost:         5,
	})

	if result.NetOldHomeProceeds != 240 {
		t.Fatalf("NetOldHomeProceeds = %v, want 240", result.NetOldHomeProceeds)
	}
	if result.PressureLevel != PressureStrained {
		t.Fatalf("PressureLevel = %q, want %q", result.PressureLevel, PressureStrained)
	}
	if result.Strategy != "先卖后买或同步推进" {
		t.Fatalf("Strategy = %q", result.Strategy)
	}
}
```

- [ ] **Step 2: Implement domain types**

Use JSON-compatible field names in DTOs, but keep domain struct names in Go:

```go
type PressureLevel string

const (
	PressureSafe     PressureLevel = "safe"
	PressureStrained PressureLevel = "strained"
	PressureDanger   PressureLevel = "danger"
)
```

The calculation formula must match `src/lib/decision.ts`.

- [ ] **Step 3: Define application repository port**

```go
type CalculationRepository interface {
	Save(ctx context.Context, record CalculationRecord) (CalculationRecord, error)
	Find(ctx context.Context, id string) (CalculationRecord, error)
}
```

`CalculationRecord` fields:

```text
ID string
Input capacity.HousingCapacityInput
Result capacity.HousingCapacityResult
CreatedAt time.Time
```

- [ ] **Step 4: Write HTTP handler tests**

`POST /api/v1/capacity/calculations` success must return:

```json
{
  "id": "calc_123",
  "result": {
    "pressureLevel": "strained",
    "strategy": "先卖后买或同步推进"
  }
}
```

Invalid JSON must return:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "request body is invalid"
  }
}
```

- [ ] **Step 5: Add OpenAPI paths**

Add schemas and paths for:

```text
POST /api/v1/capacity/calculations
GET /api/v1/capacity/calculations/{id}
```

Include every numeric input currently present in `HousingCapacityInput`.

- [ ] **Step 6: Verify task**

Run:

```bash
cd backend
go test ./internal/domain/capacity ./internal/application/capacity ./internal/interfaces/http/... -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/domain/capacity backend/internal/application/capacity backend/internal/interfaces/http backend/api/openapi.yaml
git commit -m "feat: add capacity calculation api"
```

---

### Task 5: PostgreSQL Migrations, GORM Repositories, And Migration Command

**Files:**
- Create: `backend/migrations/000001_initial_schema.up.sql`
- Create: `backend/migrations/000001_initial_schema.down.sql`
- Create: `backend/internal/infrastructure/migrate/migrate.go`
- Create: `backend/internal/infrastructure/postgres/gorm/db.go`
- Create: `backend/internal/infrastructure/postgres/gorm/models.go`
- Create: `backend/internal/infrastructure/postgres/gorm/capacity_repository.go`
- Create: `backend/internal/infrastructure/postgres/gorm/capacity_repository_test.go`
- Modify: `backend/internal/platform/app/app.go`

**Interfaces:**
- Consumes: `application/capacity.CalculationRepository`
- Produces: persistent capacity calculations.
- Produces: `propulse migrate up` and `propulse migrate down`.

- [ ] **Step 1: Write migration SQL**

`000001_initial_schema.up.sql` must create:

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE capacity_calculations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  input JSONB NOT NULL,
  result JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE neighborhoods (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  area TEXT NOT NULL DEFAULT '',
  target_layout TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE watchlist_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL DEFAULT 'demo-user',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, neighborhood_id)
);

CREATE TABLE raw_collection_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_type TEXT NOT NULL,
  source_ref TEXT NOT NULL,
  payload JSONB NOT NULL,
  collected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE listing_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  listing_price NUMERIC(12,2) NOT NULL,
  transaction_price NUMERIC(12,2),
  price_cut BOOLEAN NOT NULL DEFAULT false,
  days_on_market INT NOT NULL DEFAULT 0,
  layout TEXT NOT NULL DEFAULT '',
  captured_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE neighborhood_metrics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  listed_homes INT NOT NULL,
  price_cut_homes INT NOT NULL,
  avg_days_on_market NUMERIC(8,2) NOT NULL,
  listing_price_min NUMERIC(12,2) NOT NULL,
  listing_price_max NUMERIC(12,2) NOT NULL,
  transaction_price_min NUMERIC(12,2) NOT NULL,
  transaction_price_max NUMERIC(12,2) NOT NULL,
  transaction_momentum TEXT NOT NULL,
  target_layout_supply INT NOT NULL,
  calculated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_listing_snapshots_neighborhood_captured_at
  ON listing_snapshots(neighborhood_id, captured_at DESC);

CREATE INDEX idx_neighborhood_metrics_neighborhood_calculated_at
  ON neighborhood_metrics(neighborhood_id, calculated_at DESC);
```

- [ ] **Step 2: Add down migration**

Drop tables in reverse dependency order and leave `pgcrypto` installed:

```sql
DROP TABLE IF EXISTS neighborhood_metrics;
DROP TABLE IF EXISTS listing_snapshots;
DROP TABLE IF EXISTS raw_collection_records;
DROP TABLE IF EXISTS watchlist_items;
DROP TABLE IF EXISTS neighborhoods;
DROP TABLE IF EXISTS capacity_calculations;
```

- [ ] **Step 3: Implement migrate runner**

Use `github.com/golang-migrate/migrate/v4` with embedded or file source. `propulse migrate up` must be idempotent when already at latest version.

- [ ] **Step 4: Implement GORM capacity repository**

Store `Input` and `Result` as JSONB. Repository `Find` must return application-level not-found error mapped later to 404.

- [ ] **Step 5: Write repository integration test**

Use `PROPULSE_TEST_DATABASE_URL`. If it is empty, skip with:

```go
t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
```

Test flow:

```text
Run migrations up.
Save one capacity calculation.
Find by returned ID.
Assert result.pressureLevel equals strained.
```

- [ ] **Step 6: Wire repository into HTTP app**

`api` and `serve` modes must open PostgreSQL and use the GORM capacity repository instead of any in-memory fake.

- [ ] **Step 7: Verify task**

Run with a local database:

```bash
docker compose up -d postgres
cd backend
PROPULSE_DATABASE_URL=postgres://propulse:propulse@127.0.0.1:5432/propulse?sslmode=disable go run ./cmd/propulse migrate up
PROPULSE_TEST_DATABASE_URL=postgres://propulse:propulse@127.0.0.1:5432/propulse?sslmode=disable go test ./internal/infrastructure/... ./internal/interfaces/http/... -v
```

Expected: migrations apply once; second `migrate up` exits 0; repository tests PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/migrations backend/internal/infrastructure/migrate backend/internal/infrastructure/postgres backend/internal/platform/app
git commit -m "feat: persist capacity calculations in postgres"
```

---

### Task 6: Neighborhood Watchlist API And Seed Data

**Files:**
- Create: `backend/internal/domain/neighborhood/signal.go`
- Create: `backend/internal/domain/neighborhood/signal_test.go`
- Create: `backend/internal/application/neighborhood/ports.go`
- Create: `backend/internal/application/neighborhood/commands.go`
- Create: `backend/internal/application/neighborhood/queries.go`
- Create: `backend/internal/application/neighborhood/service_test.go`
- Create: `backend/internal/infrastructure/postgres/gorm/neighborhood_repository.go`
- Create: `backend/internal/interfaces/http/handler/neighborhood.go`
- Create: `backend/internal/interfaces/http/handler/watchlist.go`
- Modify: `backend/api/openapi.yaml`
- Modify: `backend/internal/interfaces/http/router/router.go`

**Interfaces:**
- Consumes: PostgreSQL schema from Task 5.
- Produces: `POST /api/v1/neighborhoods`
- Produces: `GET /api/v1/neighborhoods/:id`
- Produces: `GET /api/v1/neighborhoods/:id/metrics`
- Produces: `POST /api/v1/watchlist/items`
- Produces: `GET /api/v1/watchlist`

- [ ] **Step 1: Port neighborhood signal tests**

Use cases from `src/lib/decision.test.ts`:

```text
青枫花园 => status 适合砍价, supplyPressure high.
云澜府 => status 价格偏硬, supplyPressure low.
```

- [ ] **Step 2: Implement pure domain rules**

Domain types must include:

```text
TransactionMomentum: weak, stable, strong
SupplyPressure: low, medium, high
NeighborhoodStatus: 重点看, 继续观察, 适合砍价, 价格偏硬, 暂不建议追
Scarcity: low, medium, high
```

The formula must match `evaluateNeighborhoodSignal` from `src/lib/decision.ts`.

- [ ] **Step 3: Define application ports**

```go
type Repository interface {
	CreateNeighborhood(ctx context.Context, input CreateNeighborhoodInput) (Neighborhood, error)
	GetNeighborhood(ctx context.Context, id string) (Neighborhood, error)
	AddWatchlistItem(ctx context.Context, userID string, neighborhoodID string) (WatchlistItem, error)
	ListWatchlist(ctx context.Context, userID string) ([]WatchlistSummary, error)
	LatestMetric(ctx context.Context, neighborhoodID string) (MetricSnapshot, error)
}
```

- [ ] **Step 4: Implement HTTP behavior**

Use `demo-user` as the user ID until auth is implemented.

`GET /api/v1/watchlist` response shape:

```json
{
  "items": [
    {
      "id": "watch_1",
      "neighborhoodId": "neighborhood_1",
      "name": "青枫花园",
      "area": "滨江核心",
      "targetLayout": "三房",
      "status": "适合砍价",
      "listedHomes": 42,
      "priceCutHomes": 11,
      "transactionMomentum": "weak",
      "advice": "约看 500-530 万三房，尝试砍价，窗口期已打开。"
    }
  ]
}
```

- [ ] **Step 5: Add deterministic seed path for local development**

When `PROPULSE_SEED_DEMO_DATA=true`, app startup must insert:

```text
青枫花园 / 滨江核心 / 三房
云澜府 / 城东新区 / 四房
```

Use `ON CONFLICT` or repository-level idempotency so repeated startup does not duplicate rows.

- [ ] **Step 6: Verify task**

Run:

```bash
cd backend
go test ./internal/domain/neighborhood ./internal/application/neighborhood ./internal/interfaces/http/... -v
PROPULSE_SEED_DEMO_DATA=true go run ./cmd/propulse api
curl -s http://127.0.0.1:8080/api/v1/watchlist
```

Expected:

```text
tests PASS
watchlist JSON includes 青枫花园 and 云澜府
```

- [ ] **Step 7: Commit**

```bash
git add backend/internal/domain/neighborhood backend/internal/application/neighborhood backend/internal/infrastructure/postgres/gorm backend/internal/interfaces/http backend/api/openapi.yaml
git commit -m "feat: add neighborhood watchlist api"
```

---

### Task 7: Manual Import And Raw-To-Snapshot Storage

**Files:**
- Create: `backend/internal/application/collection/ports.go`
- Create: `backend/internal/application/collection/imports.go`
- Create: `backend/internal/application/collection/imports_test.go`
- Create: `backend/internal/infrastructure/postgres/gorm/collection_repository.go`
- Create: `backend/internal/interfaces/http/handler/admin_imports.go`
- Modify: `backend/internal/interfaces/http/router/router.go`
- Modify: `backend/api/openapi.yaml`

**Interfaces:**
- Consumes: `raw_collection_records` and `listing_snapshots` tables from Task 5.
- Produces: `POST /admin/api/imports`
- Produces: raw records and listing snapshots that metric jobs can read.

- [ ] **Step 1: Define import request**

Use this JSON request:

```json
{
  "sourceType": "manual_json",
  "sourceRef": "demo-weekly-import",
  "neighborhoodId": "uuid",
  "records": [
    {
      "listingPrice": 520,
      "transactionPrice": 495,
      "priceCut": true,
      "daysOnMarket": 78,
      "layout": "三房"
    }
  ]
}
```

- [ ] **Step 2: Write application test**

Test that one import with two records:

```text
Creates one raw_collection_records row.
Creates two listing_snapshots rows.
Returns importedSnapshotCount = 2.
```

- [ ] **Step 3: Implement application handler**

Validation rules:

```text
sourceType must be manual_json.
neighborhoodId must be non-empty.
records length must be between 1 and 500.
listingPrice must be greater than 0.
daysOnMarket must be greater than or equal to 0.
```

Return stable errors:

```text
invalid_request
neighborhood_not_found
import_failed
```

- [ ] **Step 4: Implement admin HTTP endpoint**

`POST /admin/api/imports` returns:

```json
{
  "collectionRunId": "uuid",
  "importedSnapshotCount": 2
}
```

- [ ] **Step 5: Verify task**

Run:

```bash
cd backend
go test ./internal/application/collection ./internal/interfaces/http/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/application/collection backend/internal/infrastructure/postgres/gorm backend/internal/interfaces/http backend/api/openapi.yaml
git commit -m "feat: add manual listing import endpoint"
```

---

### Task 8: sqlc Metric Queries And Neighborhood Metric Calculation

**Files:**
- Create: `backend/sqlc.yaml`
- Create: `backend/queries/neighborhood_metrics.sql`
- Create: `backend/internal/application/metric/ports.go`
- Create: `backend/internal/application/metric/calculate_neighborhood.go`
- Create: `backend/internal/application/metric/calculate_neighborhood_test.go`
- Create: `backend/internal/infrastructure/postgres/sqlc/db.go`
- Create: `backend/internal/infrastructure/postgres/sqlc/neighborhood_metrics.sql.go` generated by sqlc
- Create: `backend/internal/infrastructure/postgres/sqlmetric/repository.go`
- Modify: `backend/internal/application/neighborhood/queries.go`

**Interfaces:**
- Consumes: listing snapshots from Task 7.
- Produces: `metric.CalculateNeighborhood(ctx, neighborhoodID string) error`
- Produces: latest metric query consumed by watchlist and `/neighborhoods/:id/metrics`.

- [ ] **Step 1: Add sqlc config**

`backend/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "migrations"
    queries: "queries"
    gen:
      go:
        package: "sqlc"
        out: "internal/infrastructure/postgres/sqlc"
        sql_package: "pgx/v5"
```

- [ ] **Step 2: Add metric SQL**

`backend/queries/neighborhood_metrics.sql` must include:

```sql
-- name: AggregateListingSnapshots :one
SELECT
  COUNT(*)::int AS listed_homes,
  COUNT(*) FILTER (WHERE price_cut)::int AS price_cut_homes,
  COALESCE(AVG(days_on_market), 0)::numeric AS avg_days_on_market,
  COALESCE(MIN(listing_price), 0)::numeric AS listing_price_min,
  COALESCE(MAX(listing_price), 0)::numeric AS listing_price_max,
  COALESCE(MIN(transaction_price), 0)::numeric AS transaction_price_min,
  COALESCE(MAX(transaction_price), 0)::numeric AS transaction_price_max,
  COUNT(*) FILTER (WHERE layout = sqlc.arg(target_layout))::int AS target_layout_supply
FROM listing_snapshots
WHERE neighborhood_id = sqlc.arg(neighborhood_id);

-- name: InsertNeighborhoodMetric :one
INSERT INTO neighborhood_metrics (
  neighborhood_id,
  listed_homes,
  price_cut_homes,
  avg_days_on_market,
  listing_price_min,
  listing_price_max,
  transaction_price_min,
  transaction_price_max,
  transaction_momentum,
  target_layout_supply
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
)
RETURNING *;

-- name: LatestNeighborhoodMetric :one
SELECT *
FROM neighborhood_metrics
WHERE neighborhood_id = $1
ORDER BY calculated_at DESC
LIMIT 1;
```

- [ ] **Step 3: Generate sqlc code**

Run:

```bash
cd backend
go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
```

Expected: generated Go files under `backend/internal/infrastructure/postgres/sqlc`.

- [ ] **Step 4: Implement metric application service**

Transaction momentum mapping:

```text
listed_homes >= 40 and price_cut_homes/listed_homes >= 0.2 => weak
listed_homes < 20 and price_cut_homes/listed_homes < 0.1 => strong
otherwise => stable
```

The metric service must write exactly one `neighborhood_metrics` row per invocation.

- [ ] **Step 5: Verify task**

Run:

```bash
cd backend
go test ./internal/application/metric ./internal/infrastructure/postgres/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/sqlc.yaml backend/queries backend/internal/application/metric backend/internal/infrastructure/postgres/sqlc backend/internal/infrastructure/postgres/sqlmetric backend/internal/application/neighborhood
git commit -m "feat: calculate neighborhood metrics from snapshots"
```

---

### Task 9: Redis/Asynq Queue, Worker, Scheduler, And Job Logging

**Files:**
- Create: `backend/internal/application/queue/tasks.go`
- Create: `backend/internal/infrastructure/queue/client.go`
- Create: `backend/internal/infrastructure/queue/server.go`
- Create: `backend/internal/infrastructure/queue/handlers.go`
- Create: `backend/internal/infrastructure/queue/handlers_test.go`
- Modify: `backend/internal/platform/app/app.go`
- Modify: `backend/internal/infrastructure/config/config.go`

**Interfaces:**
- Consumes: `metric.CalculateNeighborhood`
- Produces: task type `metric.calculate_neighborhood`
- Produces: documented placeholders for future task types without implementing fake behavior:
  `collection.fetch_source`, `collection.normalize_listing`, `collection.deduplicate_listing`, `decision.refresh_window`, `notification.generate_weekly_report`, `notification.send_alert`.

- [ ] **Step 1: Define task payload**

```go
const TypeMetricCalculateNeighborhood = "metric.calculate_neighborhood"

type MetricCalculateNeighborhoodPayload struct {
	NeighborhoodID string `json:"neighborhoodId"`
	SourceID       string `json:"sourceId,omitempty"`
}
```

- [ ] **Step 2: Write handler test**

Test invalid payload:

```text
empty neighborhoodId returns error code invalid_task_payload
metric service is not called
```

Test valid payload:

```text
metric service receives neighborhoodID
handler log context includes task_type and source_id
```

- [ ] **Step 3: Implement Asynq client/server**

Use Redis address from `config.Config.RedisAddr`.

Queue names:

```text
critical
default
low
```

Metric tasks use `default`.

- [ ] **Step 4: Implement scheduler mode**

For first phase, scheduler enqueues metric calculation jobs for all watchlist neighborhoods every hour.

Config:

```text
PROPULSE_SCHEDULER_INTERVAL=1h
```

Local test value can be `10s`.

- [ ] **Step 5: Wire serve mode**

`serve` must start:

```text
HTTP API + embedded frontend
Asynq worker
scheduler
```

`api` must start only HTTP. `worker` must start only Asynq worker. `scheduler` must start only scheduler.

- [ ] **Step 6: Verify task**

Run:

```bash
docker compose up -d redis postgres
cd backend
go test ./internal/infrastructure/queue ./internal/platform/app -v
PROPULSE_SCHEDULER_INTERVAL=10s go run ./cmd/propulse scheduler
go run ./cmd/propulse worker
```

Expected:

```text
queue tests PASS
scheduler logs JSON with task_type=metric.calculate_neighborhood
worker logs JSON with job_id and task_type
```

- [ ] **Step 7: Commit**

```bash
git add backend/internal/application/queue backend/internal/infrastructure/queue backend/internal/platform/app backend/internal/infrastructure/config
git commit -m "feat: add redis asynq worker and scheduler"
```

---

### Task 10: Action Window API

**Files:**
- Create: `backend/internal/domain/decision/action_window.go`
- Create: `backend/internal/domain/decision/action_window_test.go`
- Create: `backend/internal/application/decision/query.go`
- Create: `backend/internal/application/decision/query_test.go`
- Create: `backend/internal/interfaces/http/handler/decision.go`
- Modify: `backend/internal/interfaces/http/router/router.go`
- Modify: `backend/api/openapi.yaml`

**Interfaces:**
- Consumes: latest capacity calculation result and latest neighborhood metric.
- Produces: `GET /api/v1/decision/action-window`
- Produces: `decision.RecommendActionWindow(input decision.ActionWindowInput) decision.ActionWindowResult`

- [ ] **Step 1: Port action-window tests**

Use cases from `src/lib/decision.test.ts`:

```text
strained + no gap + 适合砍价 + alternativesBetter => 砍价, confidence 高.
danger + gap => 等, confidence 高.
safe + 重点看 + high scarcity => 出手, confidence 中.
```

- [ ] **Step 2: Implement query composition**

First phase query inputs:

```text
Use the latest capacity calculation for demo-user.
Use the first watchlist item unless query param neighborhoodId is provided.
Use latest metric for the selected neighborhood.
Use alternativesBetter=true when watchlist count is greater than 1.
```

If missing capacity calculation, return:

```json
{
  "error": {
    "code": "capacity_required",
    "message": "create a capacity calculation before requesting an action window"
  }
}
```

- [ ] **Step 3: Implement HTTP endpoint**

`GET /api/v1/decision/action-window` returns:

```json
{
  "action": "砍价",
  "confidence": "高",
  "summary": "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。",
  "checklist": [
    "约看 3 套成交区间附近、挂牌超过 60 天的目标户型。"
  ],
  "risks": [
    "预算不是完全宽松，砍价失败时不要上调总价硬追。"
  ]
}
```

- [ ] **Step 4: Verify task**

Run:

```bash
cd backend
go test ./internal/domain/decision ./internal/application/decision ./internal/interfaces/http/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/domain/decision backend/internal/application/decision backend/internal/interfaces/http backend/api/openapi.yaml
git commit -m "feat: add action window api"
```

---

### Task 11: Frontend API Client And Page Integration

**Files:**
- Modify: `package.json`
- Create: `src/lib/api-client.ts`
- Create: `src/lib/api-client.test.ts`
- Modify: `src/components/calculator-panel.tsx`
- Modify: `src/components/watchlist-page.tsx`
- Modify: `src/components/action-window-page.tsx`
- Modify: `src/components/product-ui.test.tsx`

**Interfaces:**
- Consumes: `backend/api/openapi.yaml`
- Consumes: `/api/v1/capacity/calculations`
- Consumes: `/api/v1/watchlist`
- Consumes: `/api/v1/decision/action-window`
- Produces: frontend pages that use Go REST data while preserving current demo fallback when API is unavailable.

- [ ] **Step 1: Add OpenAPI TypeScript generation**

Add dev dependency:

```bash
pnpm add -D openapi-typescript
```

Add package script:

```json
"generate:api": "openapi-typescript backend/api/openapi.yaml -o src/lib/generated-api.d.ts"
```

- [ ] **Step 2: Create API wrapper**

`src/lib/api-client.ts` exports:

```ts
export async function createCapacityCalculation(input: HousingCapacityInput): Promise<CapacityCalculationResponse>
export async function getWatchlist(): Promise<WatchlistResponse>
export async function getActionWindow(): Promise<ActionWindowResponse>
```

All functions must:

```text
Use fetch with relative URLs.
Throw ApiError with code and message for non-2xx API responses.
Support AbortSignal as an optional final parameter.
```

- [ ] **Step 3: Write client tests**

Use mocked `global.fetch`:

```text
createCapacityCalculation posts JSON to /api/v1/capacity/calculations.
getWatchlist reads /api/v1/watchlist.
API error JSON becomes ApiError.code.
```

- [ ] **Step 4: Update calculator**

Keep local `calculateHousingCapacity` for immediate client-side preview. On "重新生成诊断报告":

```text
POST current input to Go API.
Replace displayed result with API result.
Show inline error text if API fails.
Keep existing inputs and local preview working when API is offline.
```

- [ ] **Step 5: Update watchlist page**

On mount:

```text
GET /api/v1/watchlist.
Render returned items.
Fallback to current hard-coded 青枫花园/云澜府 content when the API is unavailable.
```

- [ ] **Step 6: Update action window page**

On mount:

```text
GET /api/v1/decision/action-window.
Render action, confidence, checklist, and risks from the response.
Fallback to current static copy when API returns capacity_required or is unavailable.
```

- [ ] **Step 7: Verify task**

Run:

```bash
pnpm generate:api
pnpm typecheck
pnpm lint
pnpm test
pnpm build:web
```

Expected: all commands PASS and `backend/web/static/index.html` is refreshed.

- [ ] **Step 8: Commit**

```bash
git add package.json pnpm-lock.yaml src/lib src/components backend/web
git commit -m "feat: connect frontend pages to go api"
```

---

### Task 12: Docker Compose, Runtime Image, And End-To-End Verification

**Files:**
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`
- Modify: `README.md`
- Create: `backend/internal/platform/app/e2e_test.go`

**Interfaces:**
- Consumes: all previous tasks.
- Produces: one runtime image with one Go binary.
- Produces: Compose services `propulse`, `propulse-worker`, `propulse-scheduler`, `postgres`, `redis`.

- [ ] **Step 1: Rewrite Dockerfile as multi-stage Go runtime**

Stages:

```text
node-deps: install pnpm dependencies
frontend-builder: run pnpm build:web
go-builder: copy backend and backend/web, run go build
runner: alpine image with ca-certificates and chromium dependencies if chromedp is enabled later
```

Final command:

```dockerfile
CMD ["/usr/local/bin/propulse", "serve"]
```

- [ ] **Step 2: Update docker-compose**

Services:

```yaml
services:
  propulse:
    image: propulse:local
    command: ["/usr/local/bin/propulse", "api"]
    ports:
      - "18080:8080"
    depends_on:
      - postgres
      - redis

  propulse-worker:
    image: propulse:local
    command: ["/usr/local/bin/propulse", "worker"]
    depends_on:
      - postgres
      - redis

  propulse-scheduler:
    image: propulse:local
    command: ["/usr/local/bin/propulse", "scheduler"]
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16

  redis:
    image: redis:7
```

Use these environment defaults:

```text
POSTGRES_USER=propulse
POSTGRES_PASSWORD=propulse
POSTGRES_DB=propulse
PROPULSE_DATABASE_URL=postgres://propulse:propulse@postgres:5432/propulse?sslmode=disable
PROPULSE_REDIS_ADDR=redis:6379
PROPULSE_HTTP_ADDR=:8080
PROPULSE_SEED_DEMO_DATA=true
```

- [ ] **Step 3: Add startup migration behavior**

Compose `propulse` service command should run migrations before API startup:

```yaml
command: ["/bin/sh", "-c", "/usr/local/bin/propulse migrate up && /usr/local/bin/propulse api"]
```

Keep `worker` and `scheduler` as direct binary commands.

- [ ] **Step 4: Add README instructions**

Document:

```bash
pnpm install
pnpm verify
pnpm build:web
cd backend && go test ./...
docker compose up --build
curl http://127.0.0.1:18080/healthz
curl http://127.0.0.1:18080/api/v1/watchlist
```

Also document binary modes:

```bash
propulse serve
propulse api
propulse worker
propulse scheduler
propulse migrate up
propulse migrate down
```

- [ ] **Step 5: Add e2e smoke test**

The e2e test may be skipped unless `PROPULSE_E2E_BASE_URL` is set:

```go
func TestE2ESmoke(t *testing.T) {
	base := os.Getenv("PROPULSE_E2E_BASE_URL")
	if base == "" {
		t.Skip("PROPULSE_E2E_BASE_URL is not set")
	}

	for _, path := range []string{"/healthz", "/", "/api/v1/watchlist"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		if resp.StatusCode >= 500 {
			t.Fatalf("GET %s status = %d", path, resp.StatusCode)
		}
	}
}
```

- [ ] **Step 6: Full verification**

Run:

```bash
pnpm verify
pnpm build:web
cd backend
go test ./...
go build -o bin/propulse ./cmd/propulse
cd ..
docker compose up --build -d
curl -f http://127.0.0.1:18080/healthz
curl -f http://127.0.0.1:18080/api/v1/watchlist
cd backend
PROPULSE_E2E_BASE_URL=http://127.0.0.1:18080 go test ./internal/platform/app -run TestE2ESmoke -v
```

Expected:

```text
pnpm verify PASS
go test ./... PASS
docker compose services healthy enough for curl checks
E2E smoke test PASS
```

- [ ] **Step 7: Commit**

```bash
git add Dockerfile docker-compose.yml README.md backend/internal/platform/app
git commit -m "feat: package propulse as go integrated service"
```

---

## Self-Review

**Spec coverage:** This plan covers the first-stage scope from the technology-stack design: Go skeleton, embedded Next static export, Gin HTTP server, zerolog logging, PostgreSQL migration, Redis/Asynq, DDD Lite + CQRS + Clean Architecture directories, capacity API, watchlist API, manual import data source, neighborhood metric task, and frontend API integration.

**Deferred by design:** Full auth, permissions, notifications, weekly reports, real public-web collection, chromedp collectors, admin correction UI, and paid/authorized data-source adapters are represented by stable boundaries but not implemented in this first plan. They are outside the design document's recommended first-stage landing range.

**Placeholder scan:** The plan intentionally avoids unresolved marker text, vague "add validation" steps, and unowned future implementation notes. Every deferred area has an explicit reason and boundary.

**Type consistency:** Capacity, neighborhood, decision, queue, repository, and HTTP names are consistent across tasks. The frontend wrapper names match the API paths added to OpenAPI.

**Execution handoff:** Implement Task 1 through Task 12 in order. Do not start Task 11 before Task 4, Task 6, and Task 10 have added OpenAPI paths. Do not start Task 12 before `pnpm build:web` and `cd backend && go test ./...` pass locally.
