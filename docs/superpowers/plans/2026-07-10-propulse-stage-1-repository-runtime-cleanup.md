# Propulse Stage 1 Repository And Runtime Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate Propulse into one Node package under `apps/web`, preserve a frontend-containing `go build ./cmd/propulse`, clean undeployed migrations, protect personal/write APIs with one access token, share runtime data resources, and make readiness truthful.

**Architecture:** The tracked Next.js static export moves beside the frontend source into a Go embed package at `apps/web/embed`. The application composition root opens one process runtime containing shared GORM, pgx, Redis, repositories, and services, then passes that runtime to HTTP, worker, and scheduler components. Public market reads remain unauthenticated while all personal reads and writes use one bearer token.

**Tech Stack:** Go 1.25, Gin, GORM, pgx, go-redis, Asynq, PostgreSQL, Redis, Next.js 15 static export, pnpm 11, OpenAPI.

## Global Constraints

- A clean checkout must build a complete frontend-containing binary with `go build ./cmd/propulse` and no Node.js invocation.
- The repository has one Node package, one `pnpm-lock.yaml`, and one local `node_modules`, all under `apps/web`.
- The shared OpenAPI contract stays at `api/openapi.yaml`; repository-wide verification stays under root `scripts/`.
- All personal reads and all write endpoints require `PROPULSE_ACCESS_TOKEN` bearer authentication.
- `GET /api/v1/neighborhoods/{id}` and `GET /api/v1/neighborhoods/{id}/metrics` remain public.
- `/healthz` checks process liveness only; `/readyz` fails closed unless PostgreSQL, pgx, Redis, and access-token configuration are ready.
- The database migrations have not been deployed, so the migration history may be consolidated without backward compatibility.
- Runtime behavior changes follow test-driven development: write and observe each failing test before implementation.
- Configuration moves, generated code, lockfile normalization, and tracked static asset moves are mechanical exceptions to the test-first requirement; they must still be followed by explicit verification.

---

## File Structure

### Files Created

- `apps/web/embed/embed.go`: embeds the tracked frontend export.
- `apps/web/embed/embed_test.go`: proves expected static routes exist.
- `apps/web/scripts/sync-embed.mjs`: replaces the root frontend sync script.
- `migrations/embed_test.go`: freezes the clean undeployed migration set.
- `internal/platform/app/runtime.go`: opens, owns, and closes process-wide resources and services.
- `internal/platform/app/runtime_test.go`: verifies one runtime is shared and cleanup is correct.
- `internal/platform/app/readiness.go`: checks PostgreSQL, pgx, Redis, and token readiness.
- `internal/platform/app/readiness_test.go`: covers every readiness failure.
- `internal/infrastructure/redis/client.go`: constructs the reusable Redis readiness client.
- `internal/interfaces/http/middleware/access_auth.go`: single-user bearer authentication.
- `internal/interfaces/http/middleware/access_auth_test.go`: table-driven authentication tests.

### Files Moved

- `pnpm-lock.yaml` -> `apps/web/pnpm-lock.yaml`.
- `web/static/**` -> `apps/web/embed/static/**`.
- `web/web.go` -> `apps/web/embed/embed.go`, with package name changed to `webembed`.
- `web/web_test.go` -> `apps/web/embed/embed_test.go`, with package name changed to `webembed`.
- `scripts/sync-static-web.mjs` -> `apps/web/scripts/sync-embed.mjs`.

### Files Removed

- `pnpm-workspace.yaml`.
- `migrations/000002_listing_snapshots_collection_run.up.sql`.
- `migrations/000002_listing_snapshots_collection_run.down.sql`.
- `internal/interfaces/http/middleware/admin_auth.go` after replacement.
- `website/propulse.html` and the empty `website/` directory.
- the empty untracked `assets/` directory.
- the root local `node_modules/` directory.
- the obsolete root `web/` directory after its tracked contents move.

---

### Task 1: Freeze And Consolidate The Undeployed Migration Set

**Files:**
- Create: `migrations/embed_test.go`
- Verify: `migrations/000001_initial_schema.up.sql`
- Delete: `migrations/000002_listing_snapshots_collection_run.up.sql`
- Delete: `migrations/000002_listing_snapshots_collection_run.down.sql`

**Interfaces:**
- Consumes: `migrations.FS embed.FS`.
- Produces: one coherent migration version, `000001_initial_schema`.

- [ ] **Step 1: Write the failing migration-set test**

```go
package migrations

import (
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestEmbeddedMigrationSetIsSingleCoherentInitialSchema(t *testing.T) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	want := []string{
		"000001_initial_schema.down.sql",
		"000001_initial_schema.up.sql",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("embedded migrations = %#v, want %#v", names, want)
	}

	body, err := fs.ReadFile(FS, "000001_initial_schema.up.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, required := range []string{
		"collection_run_id UUID NOT NULL",
		"idx_listing_snapshots_neighborhood_run_captured_at",
	} {
		if !strings.Contains(string(body), required) {
			t.Fatalf("initial schema is missing %q", required)
		}
	}
}
```

- [ ] **Step 2: Run the test and verify the expected failure**

Run:

```bash
go test ./migrations -run TestEmbeddedMigrationSetIsSingleCoherentInitialSchema -v
```

Expected: FAIL because both `000002` files are still embedded.

- [ ] **Step 3: Remove the redundant second migration**

Delete:

```text
migrations/000002_listing_snapshots_collection_run.up.sql
migrations/000002_listing_snapshots_collection_run.down.sql
```

Do not remove `collection_run_id` or its index from `000001_initial_schema.up.sql`.

- [ ] **Step 4: Verify the migration package**

Run:

```bash
go test ./migrations -v
```

Expected: PASS.

- [ ] **Step 5: Commit the migration cleanup**

```bash
git add migrations
git commit -m "fix: consolidate undeployed database migrations"
```

---

### Task 2: Move The Embedded Frontend Under `apps/web`

**Files:**
- Create: `apps/web/embed/embed.go`
- Create: `apps/web/embed/embed_test.go`
- Move: `web/static/**` -> `apps/web/embed/static/**`
- Modify: `internal/interfaces/http/router/router.go`
- Modify: `internal/interfaces/http/router/router_test.go`
- Modify: `internal/platform/app/app.go`
- Delete: `web/web.go`
- Delete: `web/web_test.go`

**Interfaces:**
- Produces: `webembed.Embedded() fs.FS` from module path `github.com/sine-io/propulse/apps/web/embed`.
- Consumes: the existing tracked static export without changing its content.

- [ ] **Step 1: Add the new embed package test while the package is absent**

Create `apps/web/embed/embed_test.go`:

```go
package webembed

import (
	"io/fs"
	"testing"
)

func TestEmbeddedWebContainsAllStaticRoutes(t *testing.T) {
	embedded := Embedded()
	for _, name := range []string{
		"index.html",
		"calculator.html",
		"neighborhoods.html",
		"action-window.html",
		"methods.html",
		"templates.html",
		"watchlist.html",
		"icon.svg",
	} {
		if _, err := fs.Stat(embedded, name); err != nil {
			t.Errorf("%s missing from embedded web fs: %v", name, err)
		}
	}
}
```

- [ ] **Step 2: Run the new package test and verify it fails**

Run:

```bash
go test ./apps/web/embed -v
```

Expected: FAIL because `Embedded` and the tracked `static` directory do not exist at the new path.

- [ ] **Step 3: Move the tracked assets and implement the new package**

Move the entire tracked `web/static` tree to `apps/web/embed/static`, then create `apps/web/embed/embed.go`:

```go
package webembed

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embedded embed.FS

func Embedded() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
```

- [ ] **Step 4: Update Go imports and defaults**

Replace imports of:

```go
"github.com/sine-io/propulse/web"
```

with:

```go
webembed "github.com/sine-io/propulse/apps/web/embed"
```

Update call sites from `web.Embedded()` to `webembed.Embedded()` in:

```text
internal/platform/app/app.go
internal/interfaces/http/router/router.go
internal/interfaces/http/router/router_test.go
```

Delete the old `web/web.go`, `web/web_test.go`, and empty `web/` directory.

- [ ] **Step 5: Verify embed, router, and binary behavior**

Run:

```bash
go test ./apps/web/embed ./internal/interfaces/http/router ./internal/platform/app
go build -o /tmp/propulse-stage1 ./cmd/propulse
/tmp/propulse-stage1 --help
rm -f /tmp/propulse-stage1
```

Expected: tests PASS and help lists all documented modes.

- [ ] **Step 6: Commit the embed move**

```bash
git add apps/web/embed internal/interfaces/http/router internal/platform/app web
git commit -m "refactor: colocate embedded frontend with web app"
```

---

### Task 3: Collapse Node Tooling Into One Standalone Package

**Files:**
- Move: `pnpm-lock.yaml` -> `apps/web/pnpm-lock.yaml`
- Move: `scripts/sync-static-web.mjs` -> `apps/web/scripts/sync-embed.mjs`
- Modify: `apps/web/package.json`
- Modify: `.gitignore`
- Delete: `pnpm-workspace.yaml`
- Delete local only: root `node_modules/`

**Interfaces:**
- Produces: `pnpm --dir apps/web install --frozen-lockfile` and `pnpm --dir apps/web build:web` without a workspace root.
- Produces: `apps/web/embed/static` as the synchronized tracked export.

- [ ] **Step 1: Move and normalize the pnpm lockfile**

Move the root lockfile to `apps/web/pnpm-lock.yaml`. Change its importer key from:

```yaml
importers:
  apps/web:
```

to:

```yaml
importers:
  .:
```

Delete `pnpm-workspace.yaml`.

- [ ] **Step 2: Move and update the frontend sync script**

Create `apps/web/scripts/sync-embed.mjs` with paths relative to `apps/web`:

```js
import { cp, mkdir, rm, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const webRoot = resolve(import.meta.dirname, "..");
const source = resolve(webRoot, "out");
const target = resolve(webRoot, "embed", "static");

await rm(target, { recursive: true, force: true });
await mkdir(target, { recursive: true });
await cp(source, target, { recursive: true });
await writeFile(resolve(target, ".gitkeep"), "");
```

Delete `scripts/sync-static-web.mjs`.

- [ ] **Step 3: Point package scripts at the local frontend script**

Change `apps/web/package.json` scripts to:

```json
{
  "generate:api": "openapi-typescript ../../api/openapi.yaml -o src/lib/generated-api.d.ts",
  "build:web": "pnpm build && node scripts/sync-embed.mjs",
  "verify:stack": "bash ../../scripts/verify-stack.sh"
}
```

Keep all other existing scripts unchanged.

- [ ] **Step 4: Remove the duplicate local dependency tree and verify standalone install**

Remove only ignored generated directories:

```bash
rm -rf node_modules apps/web/node_modules apps/web/.next apps/web/out apps/web/tsconfig.tsbuildinfo
pnpm --dir apps/web install --frozen-lockfile
```

Expected: only `apps/web/node_modules` is created.

Verify:

```bash
test ! -e node_modules
test -d apps/web/node_modules
test ! -e pnpm-workspace.yaml
test -f apps/web/pnpm-lock.yaml
```

- [ ] **Step 5: Rebuild the tracked export and verify Go remains Node-independent**

Run:

```bash
pnpm --dir apps/web verify
pnpm --dir apps/web build:web
go test ./apps/web/embed ./internal/interfaces/http/router
go build -o /tmp/propulse-stage1 ./cmd/propulse
rm -f /tmp/propulse-stage1
```

Expected: all commands PASS.

- [ ] **Step 6: Update ignore rules for the single package**

Keep recursive ignore patterns for `.next/`, `node_modules/`, `out/`, and
`*.tsbuildinfo`. Remove only the obsolete `backend/bin/` line from `.gitignore`.

- [ ] **Step 7: Commit Node-tooling consolidation**

```bash
git add .gitignore apps/web/package.json apps/web/pnpm-lock.yaml apps/web/scripts apps/web/embed/static pnpm-lock.yaml pnpm-workspace.yaml scripts/sync-static-web.mjs
git commit -m "refactor: make web app a standalone node package"
```

---

### Task 4: Replace Admin Authentication With One Access Token

**Files:**
- Create: `internal/interfaces/http/middleware/access_auth.go`
- Create: `internal/interfaces/http/middleware/access_auth_test.go`
- Modify: `internal/infrastructure/config/config.go`
- Modify: `internal/infrastructure/config/config_test.go`
- Modify: `internal/interfaces/http/router/router.go`
- Modify: `internal/interfaces/http/router/router_test.go`
- Delete: `internal/interfaces/http/middleware/admin_auth.go`

**Interfaces:**
- Produces: `middleware.AccessAuth(token string) gin.HandlerFunc`.
- Produces: `config.Config.AccessToken string`, loaded only from `PROPULSE_ACCESS_TOKEN`.
- Produces: `router.Dependencies.AccessToken string`.

- [ ] **Step 1: Write failing configuration tests for the new environment variable**

Replace the admin-token assertions with:

```go
func TestLoadReadsAccessToken(t *testing.T) {
	t.Setenv("PROPULSE_ACCESS_TOKEN", "secret-token")
	t.Setenv("PROPULSE_ADMIN_API_TOKEN", "legacy-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AccessToken != "secret-token" {
		t.Fatalf("AccessToken = %q, want secret-token", cfg.AccessToken)
	}
}

func TestLoadDoesNotAcceptLegacyAdminToken(t *testing.T) {
	t.Setenv("PROPULSE_ACCESS_TOKEN", "")
	t.Setenv("PROPULSE_ADMIN_API_TOKEN", "legacy-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AccessToken != "" {
		t.Fatalf("AccessToken = %q, want empty", cfg.AccessToken)
	}
}
```

Update `TestLoadUsesDocumentedDefaults` to expect `AccessToken == ""`.

- [ ] **Step 2: Run config tests and verify they fail**

Run:

```bash
go test ./internal/infrastructure/config -v
```

Expected: FAIL because `Config.AccessToken` does not exist.

- [ ] **Step 3: Implement the configuration rename**

Replace:

```go
AdminAPIToken string
```

with:

```go
AccessToken string
```

and load it with:

```go
AccessToken: getEnv("PROPULSE_ACCESS_TOKEN", ""),
```

Do not read `PROPULSE_ADMIN_API_TOKEN`.

- [ ] **Step 4: Write the failing access middleware tests**

Create a table-driven test with these cases:

```go
tests := []struct {
	name       string
	configured string
	header     string
	wantStatus int
}{
	{name: "missing configuration", configured: "", header: "Bearer secret", wantStatus: http.StatusUnauthorized},
	{name: "missing header", configured: "secret", header: "", wantStatus: http.StatusUnauthorized},
	{name: "wrong scheme", configured: "secret", header: "Basic secret", wantStatus: http.StatusUnauthorized},
	{name: "empty bearer", configured: "secret", header: "Bearer ", wantStatus: http.StatusUnauthorized},
	{name: "wrong token", configured: "secret", header: "Bearer wrong", wantStatus: http.StatusUnauthorized},
	{name: "valid token", configured: "secret", header: "Bearer secret", wantStatus: http.StatusNoContent},
}
```

For every failure assert:

```go
rec.Header().Get("WWW-Authenticate") == "Bearer"
body error code == "access_required"
```

- [ ] **Step 5: Run middleware tests and verify they fail**

Run:

```bash
go test ./internal/interfaces/http/middleware -run TestAccessAuth -v
```

Expected: FAIL because `AccessAuth` is undefined.

- [ ] **Step 6: Implement constant-time access authentication**

Implement:

```go
func AccessAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		const prefix = "Bearer "
		auth := c.GetHeader("Authorization")
		valid := strings.TrimSpace(token) != "" &&
			strings.HasPrefix(auth, prefix) &&
			strings.TrimSpace(strings.TrimPrefix(auth, prefix)) != "" &&
			subtle.ConstantTimeCompare(
				[]byte(strings.TrimSpace(strings.TrimPrefix(auth, prefix))),
				[]byte(token),
			) == 1
		if !valid {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code": "access_required",
				"message": "valid bearer access token is required",
			}})
			return
		}
		c.Next()
	}
}
```

Delete `AdminAuth` after router call sites move.

- [ ] **Step 7: Write failing router protection tests**

Add a table-driven test that sends requests without authorization to:

```text
POST /api/v1/capacity/calculations
GET  /api/v1/capacity/calculations/calculation_1
POST /api/v1/neighborhoods
POST /api/v1/watchlist/items
GET  /api/v1/watchlist
GET  /api/v1/decision/action-window
POST /admin/api/imports
```

Construct the router with `AccessToken: "secret-token"`. Assert every response
is `401`, and use application stubs with call counters to assert handlers were
not invoked.

Add a second test asserting unauthenticated requests still reach:

```text
GET /api/v1/neighborhoods/neighborhood_1
GET /api/v1/neighborhoods/neighborhood_1/metrics
```

- [ ] **Step 8: Run router tests and verify they fail**

Run:

```bash
go test ./internal/interfaces/http/router -run 'TestProtected|TestPublic' -v
```

Expected: FAIL because personal API routes are not protected and
`Dependencies.AccessToken` does not exist.

- [ ] **Step 9: Split public and protected route groups**

Use this route shape in `router.New`:

```go
api := engine.Group("/api/v1")
protected := api.Group("")
protected.Use(httpmiddleware.AccessAuth(deps.AccessToken))

protected.POST("/capacity/calculations", capacityHandler.CreateCalculation)
protected.GET("/capacity/calculations/:id", capacityHandler.GetCalculation)
protected.POST("/neighborhoods", neighborhoodHandler.CreateNeighborhood)
api.GET("/neighborhoods/:id", neighborhoodHandler.GetNeighborhood)
api.GET("/neighborhoods/:id/metrics", neighborhoodHandler.GetMetrics)
protected.POST("/watchlist/items", watchlistHandler.AddItem)
protected.GET("/watchlist", watchlistHandler.List)
protected.GET("/decision/action-window", decisionHandler.GetActionWindow)

admin := engine.Group("/admin/api")
admin.Use(httpmiddleware.AccessAuth(deps.AccessToken))
admin.POST("/imports", adminImportsHandler.CreateImport)
```

- [ ] **Step 10: Verify config, middleware, and router tests**

Run:

```bash
go test ./internal/infrastructure/config ./internal/interfaces/http/middleware ./internal/interfaces/http/router -v
```

Expected: PASS.

- [ ] **Step 11: Commit access-token authentication**

```bash
git add internal/infrastructure/config internal/interfaces/http/middleware internal/interfaces/http/router
git commit -m "feat: protect personal api with access token"
```

---

### Task 5: Use A Stable Single-User Identity

**Files:**
- Create: `internal/application/user/user.go`
- Modify: `internal/application/decision/query.go`
- Modify: `internal/application/decision/query_test.go`
- Modify: `internal/interfaces/http/handler/capacity.go`
- Modify: `internal/interfaces/http/handler/capacity_test.go`
- Modify: `internal/interfaces/http/handler/watchlist.go`
- Modify: `internal/interfaces/http/handler/neighborhood_test.go`
- Modify: `internal/infrastructure/postgres/gorm/neighborhood_repository.go`
- Modify: `internal/infrastructure/postgres/gorm/neighborhood_repository_test.go`
- Modify: `internal/application/capacity/service_test.go`
- Modify: `internal/application/neighborhood/service_test.go`
- Modify: `internal/interfaces/http/router/router_test.go`
- Modify: `migrations/000001_initial_schema.up.sql`

**Interfaces:**
- Produces: `user.SingleUserID = "propulse-user"`.
- Removes: production references to `demo-user` and package-local demo user constants.

- [ ] **Step 1: Write a failing application identity test**

Create `internal/application/user/user.go` only after first adding a temporary
test file or extending an existing decision test to assert:

```go
if capacity.userID != user.SingleUserID {
	t.Fatalf("capacity userID = %q, want %q", capacity.userID, user.SingleUserID)
}
if neighborhood.watchlistUserID != user.SingleUserID {
	t.Fatalf("watchlist userID = %q, want %q", neighborhood.watchlistUserID, user.SingleUserID)
}
```

Run the decision test before implementation; it must fail because the package
or constant does not exist.

- [ ] **Step 2: Add the shared single-user constant**

Create:

```go
package user

const SingleUserID = "propulse-user"
```

- [ ] **Step 3: Replace all production demo-user constants**

Import `github.com/sine-io/propulse/internal/application/user` and use
`user.SingleUserID` in:

```text
internal/application/decision/query.go
internal/interfaces/http/handler/capacity.go
internal/interfaces/http/handler/watchlist.go
internal/infrastructure/postgres/gorm/neighborhood_repository.go
```

Update the migration defaults:

```sql
user_id TEXT NOT NULL DEFAULT 'propulse-user'
```

for `capacity_calculations` and `watchlist_items`.

- [ ] **Step 4: Update tests and remove stale terminology**

Replace expected literal `demo-user` values with `user.SingleUserID` throughout
Go tests. Rename test names such as `UsesDemoUser` to `UsesSingleUser`.

Run:

```bash
rg -n "demo-user|demoUserID|UsesDemoUser" --glob '*.go' --glob '*.sql' internal migrations
```

Expected: no output.

- [ ] **Step 5: Verify identity-dependent packages**

Run:

```bash
go test ./internal/application/decision ./internal/interfaces/http/handler ./internal/infrastructure/postgres/gorm ./internal/interfaces/http/router
```

Expected: PASS; PostgreSQL integration tests may skip when
`PROPULSE_TEST_DATABASE_URL` is unset.

- [ ] **Step 6: Commit the stable identity**

```bash
git add internal migrations/000001_initial_schema.up.sql
git commit -m "refactor: use stable single user identity"
```

---

### Task 6: Open And Share One Process Runtime

**Files:**
- Create: `internal/platform/app/runtime.go`
- Create: `internal/platform/app/runtime_test.go`
- Create: `internal/infrastructure/redis/client.go`
- Modify: `internal/platform/app/app.go`
- Modify: `internal/platform/app/app_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `openRuntime(ctx context.Context, cfg config.Config, log zerolog.Logger) (*runtime, error)`.
- Produces: `(*runtime).Close() error`.
- Changes internal runners to consume one `*runtime`.

- [ ] **Step 1: Write failing runtime composition tests**

Add tests for these contracts:

```go
func TestRunAPIModeOpensAndClosesOneRuntime(t *testing.T)
func TestRunServeModeSharesOneRuntimeAcrossHTTPWorkerScheduler(t *testing.T)
func TestRunMigrateModeDoesNotOpenRuntime(t *testing.T)
```

Use one injectable factory:

```go
var openRuntimeFunc = openRuntime
```

and injectable component functions with signatures:

```go
func(context.Context, config.Config, zerolog.Logger, *runtime) error
```

The serve-mode test must record the pointer received by HTTP, worker, and
scheduler, and assert all three pointers are identical.

- [ ] **Step 2: Run the tests and verify the expected failure**

Run:

```bash
go test ./internal/platform/app -run 'TestRun(APIModeOpens|ServeModeShares|MigrateModeDoes)' -v
```

Expected: FAIL because `runtime` and `openRuntimeFunc` do not exist.

- [ ] **Step 3: Define the runtime container**

Create `runtime.go` with:

```go
type runtime struct {
	gormDB      *gorm.DB
	sqlDB       *sql.DB
	pgxPool     *pgxpool.Pool
	redis       *redis.Client
	queueClient io.Closer

	capacity     CapacityApplication
	neighborhood NeighborhoodApplication
	collection   CollectionApplication
	metric       MetricApplication
	enqueuer     MetricTaskEnqueuer
	readiness    router.ReadinessChecker
}
```

`openRuntime` performs exactly:

```go
gormDB, sqlDB, err := postgresgorm.Open(cfg.DatabaseURL)
pgxPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
redisClient := infrastructureredis.New(cfg.RedisAddr)
```

Create `internal/infrastructure/redis/client.go` with:

```go
package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func New(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}

type PingClient struct{ client *redis.Client }

func NewPingClient(client *redis.Client) PingClient {
	return PingClient{client: client}
}

func (p PingClient) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
```

Construct one each of:

```go
capacityRepo := postgresgorm.NewCapacityRepository(gormDB)
metricRepo := sqlmetric.NewRepository(pgxPool)
neighborhoodRepo := postgresgorm.NewNeighborhoodRepositoryWithMetricReader(gormDB, metricRepo)
collectionRepo := postgresgorm.NewCollectionRepository(gormDB)

capacityService := appcapacity.NewService(capacityRepo, time.Now, nil)
metricService := appmetric.NewService(metricRepo)
neighborhoodService := appneighborhood.NewService(neighborhoodRepo)
collectionService := appcollection.NewService(collectionRepo, time.Now, nil, metricService)
queueClient := infrastructurequeue.NewClient(cfg.RedisAddr)
```

Retain demo seeding only while it exists in the current stage, but seed through
the single shared `neighborhoodRepo`.

Use injectable openers so construction and failure cleanup can be tested
without live infrastructure:

```go
var openPostgres = postgresgorm.Open
var openPGXPool = pgxpool.New
var openRedisClient = infrastructureredis.New
var openQueueClient = func(addr string) (MetricTaskEnqueuer, io.Closer) {
	client := infrastructurequeue.NewClient(addr)
	return client, client
}
```

Add failure-path tests that force each opener to fail in turn and assert every
already-opened resource is closed before `openRuntime` returns.

- [ ] **Step 4: Implement deterministic runtime cleanup**

`Close` must close each owned resource once and return the first error:

```go
func (r *runtime) Close() error {
	var firstErr error
	if r.queueClient != nil {
		firstErr = errors.Join(firstErr, r.queueClient.Close())
	}
	if r.redis != nil {
		firstErr = errors.Join(firstErr, r.redis.Close())
	}
	if r.pgxPool != nil {
		r.pgxPool.Close()
	}
	if r.sqlDB != nil {
		firstErr = errors.Join(firstErr, r.sqlDB.Close())
	}
	return firstErr
}
```

Avoid double-closing a shared Redis client: the Asynq queue client remains its
own `queueClient` closer; the readiness Redis client is a separate owned
object. Set both `enqueuer: queueClient` and `queueClient: queueClient` when
constructing the runtime.

- [ ] **Step 5: Refactor `Run` and component runners**

For `api`, `worker`, `scheduler`, and `serve`:

```go
rt, err := openRuntimeFunc(ctx, cfg, log)
if err != nil {
	return err
}
defer rt.Close()
```

Use these signatures:

```go
func runHTTPServer(ctx context.Context, cfg config.Config, log zerolog.Logger, rt *runtime) error
func runQueueWorker(ctx context.Context, cfg config.Config, log zerolog.Logger, rt *runtime) error
func runScheduler(ctx context.Context, cfg config.Config, log zerolog.Logger, rt *runtime) error
```

Delete the four independent factories:

```text
openCapacityApplication
openNeighborhoodApplication
openCollectionApplication
openMetricApplication
```

The HTTP router receives services and readiness from `rt`. The worker receives
`rt.metric`. The scheduler receives `rt.neighborhood` and `rt.enqueuer`.

- [ ] **Step 6: Update existing app tests to inject one runtime**

Replace per-application factory stubs with:

```go
openRuntimeFunc = func(context.Context, config.Config, zerolog.Logger) (*runtime, error) {
	return &runtime{
		capacity:     capacityStub,
		neighborhood: neighborhoodStub,
		collection:   collectionStub,
		metric:       metricStub,
		enqueuer:     queueStub,
		queueClient:  noopCloser{},
		readiness:    readyStub{err: nil},
	}, nil
}
```

Keep mode-composition, graceful shutdown, scheduler enqueue, and injected
handler behavior tests intact.

- [ ] **Step 7: Promote go-redis to a direct dependency**

Run:

```bash
go get github.com/redis/go-redis/v9@v9.14.1
go mod tidy
```

Expected: `github.com/redis/go-redis/v9 v9.14.1` appears in the direct `require`
block.

- [ ] **Step 8: Verify runtime composition**

Run:

```bash
go test ./internal/platform/app ./internal/infrastructure/postgres/... ./internal/infrastructure/queue/... ./internal/infrastructure/redis -v
```

Expected: PASS.

- [ ] **Step 9: Commit shared runtime composition**

```bash
git add go.mod go.sum internal/platform/app internal/infrastructure/redis
git commit -m "refactor: share process runtime resources"
```

---

### Task 7: Make `/readyz` Check Real Dependencies

**Files:**
- Create: `internal/platform/app/readiness.go`
- Create: `internal/platform/app/readiness_test.go`
- Modify: `internal/infrastructure/redis/client.go`
- Modify: `internal/interfaces/http/router/router.go`
- Modify: `internal/interfaces/http/router/router_test.go`
- Modify: `internal/platform/app/runtime.go`

**Interfaces:**
- Produces: `router.ReadinessChecker` with `Check(context.Context) error`.
- Produces: `app.NewReadinessChecker(sqlPinger, pgxPinger, redisPinger, accessToken)`.

- [ ] **Step 1: Write failing router readiness tests**

Define a router test stub:

```go
type readinessStub struct{ err error }

func (s readinessStub) Check(context.Context) error { return s.err }
```

Add tests:

```go
func TestReadyRouteReturnsOKWhenDependenciesAreReady(t *testing.T)
func TestReadyRouteReturnsServiceUnavailableWhenDependencyFails(t *testing.T)
func TestReadyRouteFailsClosedWithoutChecker(t *testing.T)
func TestHealthRouteRemainsOKWhenReadinessFails(t *testing.T)
```

Failure response must be:

```json
{
  "error": {
    "code": "not_ready",
    "message": "service dependencies are not ready"
  }
}
```

- [ ] **Step 2: Run router readiness tests and verify they fail**

Run:

```bash
go test ./internal/interfaces/http/router -run 'Test(Ready|Health)' -v
```

Expected: FAIL because `/readyz` always returns 200 and no checker exists.

- [ ] **Step 3: Add the router readiness interface and fail-closed handler**

Add:

```go
type ReadinessChecker interface {
	Check(context.Context) error
}
```

to router dependencies, then implement `/readyz` using a 2-second request
timeout. Return 503 without exposing the underlying error.

- [ ] **Step 4: Write failing composite readiness tests**

Use small fakes with these interfaces:

```go
type SQLPinger interface { PingContext(context.Context) error }
type PGXPinger interface { Ping(context.Context) error }
type RedisPinger interface { Ping(context.Context) error }
```

Test success and each independent failure:

```text
database/sql ping fails
pgx ping fails
Redis ping fails
access token is empty or whitespace
```

- [ ] **Step 5: Run the composite checker tests and verify they fail**

Run:

```bash
go test ./internal/platform/app -run TestReadinessChecker -v
```

Expected: FAIL because the checker is undefined.

- [ ] **Step 6: Implement the checker**

Implement sequential checks with wrapped internal errors:

```go
func (c readinessChecker) Check(ctx context.Context) error {
	if strings.TrimSpace(c.accessToken) == "" {
		return errors.New("access token is not configured")
	}
	if err := c.sql.PingContext(ctx); err != nil {
		return fmt.Errorf("sql ping: %w", err)
	}
	if err := c.pgx.Ping(ctx); err != nil {
		return fmt.Errorf("pgx ping: %w", err)
	}
	if err := c.redis.Ping(ctx); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}
```

Use the `PingClient` adapter created in Task 6:

```go
type PingClient struct{ client *redis.Client }

func NewPingClient(client *redis.Client) PingClient {
	return PingClient{client: client}
}

func (p PingClient) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
```

Wire the runtime's `sqlDB`, `pgxPool`, Redis client, and `cfg.AccessToken` into
the checker.

- [ ] **Step 7: Verify readiness behavior**

Run:

```bash
go test ./internal/interfaces/http/router ./internal/platform/app -v
```

Expected: PASS.

- [ ] **Step 8: Commit truthful readiness**

```bash
git add internal/infrastructure/redis internal/interfaces/http/router internal/platform/app
git commit -m "feat: check dependencies in readiness endpoint"
```

---

### Task 8: Update The API Contract For Access Protection

**Files:**
- Modify: `api/openapi.yaml`
- Regenerate: `apps/web/src/lib/generated-api.d.ts`

**Interfaces:**
- Produces: OpenAPI security scheme `AccessBearerAuth`.
- Consumes: the protected/public route boundary established in Task 4.

- [ ] **Step 1: Update the OpenAPI security scheme**

Replace:

```yaml
AdminBearerAuth:
```

with:

```yaml
AccessBearerAuth:
  type: http
  scheme: bearer
```

Add:

```yaml
security:
  - AccessBearerAuth: []
```

to every protected operation from Task 4. Add a structured `401` response to
each. Remove the obsolete admin-specific `403` response and all `demo user`
phrasing.

- [ ] **Step 2: Validate the contract textually before generation**

Run:

```bash
rg -n "AdminBearerAuth|demo user|Admin bearer|admin authorization" api/openapi.yaml
```

Expected: no output.

Run:

```bash
rg -n "AccessBearerAuth" api/openapi.yaml
```

Expected: the security scheme and seven protected operations are present.

- [ ] **Step 3: Regenerate TypeScript API types**

Run:

```bash
pnpm --dir apps/web generate:api
pnpm --dir apps/web typecheck
```

Expected: both commands PASS.

- [ ] **Step 4: Commit the contract update**

```bash
git add api/openapi.yaml apps/web/src/lib/generated-api.d.ts
git commit -m "docs: describe access protected api"
```

---

### Task 9: Update Docker, Compose, Smoke Tests, And Documentation

**Files:**
- Modify: `Dockerfile`
- Modify: `.dockerignore`
- Modify: `docker-compose.yml`
- Modify: `scripts/verify-stack.sh`
- Modify: `internal/platform/app/e2e_test.go`
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-07-08-propulse-go-integrated-backend-design.md`
- Modify: `docs/superpowers/plans/2026-07-09-propulse-go-integrated-backend-implementation.md`

**Interfaces:**
- Produces: Docker builds from the standalone `apps/web` package.
- Produces: local access token `local-access-token` for Compose and smoke tests.
- Produces: historical-document warnings for obsolete path descriptions.

- [ ] **Step 1: Update Docker build inputs for the standalone frontend**

Use this Node dependency stage shape:

```dockerfile
FROM node:22-alpine AS node-deps
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm install -g pnpm@11.8.0
WORKDIR /app/apps/web
COPY apps/web/package.json apps/web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM node-deps AS frontend-builder
WORKDIR /app
COPY . .
RUN pnpm --dir apps/web build:web
```

In the Go builder copy `apps/web/embed` instead of root `web`:

```dockerfile
COPY apps/web/embed ./apps/web/embed
```

Do not duplicate-copy the export from the frontend stage after `build:web` has
already refreshed `/app/apps/web/embed/static`; either copy the refreshed
directory from `frontend-builder` or copy all Go inputs from that stage.

- [ ] **Step 2: Remove unused Chromium runtime dependencies**

Replace the runner package installation with:

```dockerfile
RUN apk add --no-cache ca-certificates
```

Remove Chromium, freetype, harfbuzz, nss, and ttf-freefont.

- [ ] **Step 3: Update Docker ignore rules**

Keep `.dockerignore` exclusions for recursive Node/build output. Remove the
obsolete `assets` and `website` lines after those directories are deleted.

- [ ] **Step 4: Update Compose token configuration**

Replace every:

```yaml
PROPULSE_ADMIN_API_TOKEN: local-admin-token
```

with:

```yaml
PROPULSE_ACCESS_TOKEN: local-access-token
```

- [ ] **Step 5: Write failing E2E expectations for protected routes**

Update `TestE2ESmoke` to:

```go
token := os.Getenv("PROPULSE_E2E_ACCESS_TOKEN")
if token == "" {
	t.Skip("PROPULSE_E2E_ACCESS_TOKEN is not set")
}

for _, path := range []string{"/healthz", "/readyz", "/"} {
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
	}
}

unauthorized, err := http.Get(base + "/api/v1/watchlist")
if err != nil {
	t.Fatalf("unauthenticated watchlist request failed: %v", err)
}
unauthorized.Body.Close()
if unauthorized.StatusCode != http.StatusUnauthorized {
	t.Fatalf("unauthenticated watchlist status = %d, want 401", unauthorized.StatusCode)
}

req, err := http.NewRequest(http.MethodGet, base+"/api/v1/watchlist", nil)
if err != nil {
	t.Fatalf("NewRequest() error = %v", err)
}
req.Header.Set("Authorization", "Bearer "+token)
authorized, err := http.DefaultClient.Do(req)
if err != nil {
	t.Fatalf("authorized watchlist request failed: %v", err)
}
authorized.Body.Close()
if authorized.StatusCode != http.StatusOK {
	t.Fatalf("authorized watchlist status = %d, want 200", authorized.StatusCode)
}
```

Before updating the smoke script, run the test against a current stack or
router fixture and confirm the protected-route assertion fails under old
behavior.

- [ ] **Step 6: Update the full-stack verification script**

After Compose starts, poll readiness rather than assuming immediate startup:

```bash
for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:18080/readyz >/dev/null; then
    break
  fi
  sleep 1
done

curl -fsS http://127.0.0.1:18080/healthz >/dev/null
curl -fsS http://127.0.0.1:18080/readyz >/dev/null
curl -fsS \
  -H "Authorization: Bearer local-access-token" \
  http://127.0.0.1:18080/api/v1/watchlist >/dev/null

PROPULSE_E2E_BASE_URL=http://127.0.0.1:18080 \
PROPULSE_E2E_ACCESS_TOKEN=local-access-token \
go test ./internal/platform/app -run TestE2ESmoke -v
```

- [ ] **Step 7: Update current developer documentation**

README commands become:

```bash
pnpm --dir apps/web install --frozen-lockfile
pnpm --dir apps/web verify
pnpm --dir apps/web build:web
go build ./cmd/propulse
```

Document `PROPULSE_ACCESS_TOKEN`, authenticated curl examples, `/healthz`, and
`/readyz`. State explicitly that the tracked export under `apps/web/embed/static`
allows a clean Go-only build.

Add this warning immediately after the title of each obsolete 2026-07-08/09
architecture implementation document:

```markdown
> Historical document: this records the original implementation path. Current
> repository paths and security behavior are documented in
> `2026-07-10-propulse-trusted-decision-system-design.md`.
```

Do not mechanically rewrite the historical step-by-step plan.

- [ ] **Step 8: Verify Dockerfile and documentation references**

Run:

```bash
rg -n "pnpm-workspace|PROPULSE_ADMIN_API_TOKEN|local-admin-token|COPY web|/web/static" Dockerfile docker-compose.yml README.md scripts .dockerignore
```

Expected: no output.

- [ ] **Step 9: Commit integration configuration**

```bash
git add Dockerfile .dockerignore docker-compose.yml README.md scripts/verify-stack.sh internal/platform/app/e2e_test.go docs/superpowers
git commit -m "chore: align deployment with cleaned runtime"
```

---

### Task 10: Remove Obsolete Prototype And Run The Stage Gate

**Files:**
- Delete: `website/propulse.html`
- Delete local only: empty `website/`, empty `assets/`, root `node_modules/`
- Verify all Stage 1 files.

**Interfaces:**
- Produces: clean root directory with no frontend-only files outside `apps/web`, except shared `api/openapi.yaml`.

- [ ] **Step 1: Confirm the obsolete prototype has no consumers**

Run:

```bash
rg -n "website/propulse\.html|propulse\.html" --glob '!website/propulse.html' .
```

Expected: no output.

- [ ] **Step 2: Delete obsolete and ignored local artifacts**

Remove:

```text
website/propulse.html
website/
assets/
node_modules/
```

Do not delete `apps/web/node_modules`; it is now the one Node dependency tree.

- [ ] **Step 3: Verify the root boundary**

Run:

```bash
test ! -e pnpm-lock.yaml
test ! -e pnpm-workspace.yaml
test ! -e node_modules
test ! -e web
test ! -e website
test ! -e assets
test -f apps/web/package.json
test -f apps/web/pnpm-lock.yaml
test -d apps/web/node_modules
test -f apps/web/embed/embed.go
test -f apps/web/embed/static/index.html
```

Expected: all assertions succeed.

- [ ] **Step 4: Run the complete non-Docker verification gate**

Run in this order:

```bash
pnpm --dir apps/web verify
pnpm --dir apps/web build:web
git diff --exit-code -- apps/web/embed/static
go test ./...
go vet ./...
go build -o /tmp/propulse-stage1 ./cmd/propulse
/tmp/propulse-stage1 --help
rm -f /tmp/propulse-stage1
git diff --check
```

Expected:

```text
frontend typecheck, lint, and tests pass
frontend production export succeeds
tracked embed output is current
all Go tests pass or explicitly skip external PostgreSQL tests
go vet exits 0
single binary builds and prints documented modes
no whitespace errors
```

- [ ] **Step 5: Run the Docker/full-stack gate when Docker is available**

Because migration history was consolidated, destroy only the development
Compose volumes:

```bash
docker compose down -v
bash scripts/verify-stack.sh
```

Then verify explicitly:

```bash
curl -fsS http://127.0.0.1:18080/healthz
curl -fsS http://127.0.0.1:18080/readyz
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18080/api/v1/watchlist
curl -fsS -H 'Authorization: Bearer local-access-token' http://127.0.0.1:18080/api/v1/watchlist
```

Expected status for unauthenticated watchlist: `401`. If the environment lacks
Docker socket permission, record that limitation and do not claim this gate
passed.

- [ ] **Step 6: Review the final diff for scope**

Run:

```bash
git status --short
git diff --stat HEAD~9
git diff --check
rg -n "PROPULSE_ADMIN_API_TOKEN|AdminAPIToken|AdminAuth|demo-user|pnpm-workspace" --glob '!docs/superpowers/plans/2026-07-09-propulse-go-integrated-backend-implementation.md' .
```

Expected: only intentional Stage 1 changes; the final search has no active-code
or current-document matches.

- [ ] **Step 7: Commit final cleanup**

```bash
git add -A
git commit -m "chore: remove obsolete frontend prototype"
```

---

## Post-Stage Review

After Task 10:

- review the Stage 1 diff for behavior regressions;
- confirm the repository contains one Node package and a Go-only build path;
- confirm protected APIs intentionally return `401` until the Stage 3 frontend
  unlock UI is implemented;
- write the Stage 2 trusted-manual-market-data implementation plan from
  `docs/superpowers/specs/2026-07-10-propulse-trusted-decision-system-design.md`;
- do not start automatic collection or decision-review schema work during this
  stage.
