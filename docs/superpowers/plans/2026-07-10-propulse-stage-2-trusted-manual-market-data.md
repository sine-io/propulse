# Propulse Stage 2 Trusted Manual Market Data Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace prototype market snapshots with transactional, traceable JSON/CSV collection runs and expose provenance-aware public market and history APIs that never turn partial, stale, expired, or insufficient data into a confident buying signal.

**Architecture:** Protected admin adapters normalize JSON and CSV into one collection command. GORM persists the source, run, raw input, and separate listing and transaction observations in one idempotent transaction; sqlc/pgx derives retry-safe market metrics from committed observations. Public neighborhood queries combine stored metric provenance with dynamically recalculated freshness and domain quality gates, while the existing worker and scheduler keep derived metrics refreshable.

**Tech Stack:** Go 1.25, Gin, GORM, pgx/sqlc 1.31.1, PostgreSQL 16, Redis/Asynq, OpenAPI 3.0, Next.js 15 static export, pnpm 11.

## Global Constraints

- Stage 2 implements trusted manual market-data ingestion and backend read models only. Do not build the `/data` page, unlock UI, or replace runtime frontend sample data; those belong to Stage 3.
- Do not add a website collector, browser automation, Chromium dependency, scheduled scraping, or source-specific collection logic. Future collectors must submit the same normalized collection command.
- Keep the repository at one Node package under `apps/web`; a clean checkout must still build a frontend-containing binary with `go build ./cmd/propulse` and no Node.js invocation.
- Keep the single undeployed migration pair. Rewrite `000001_initial_schema` coherently; do not add `000002` compatibility migrations.
- Because `000001` is intentionally rewritten, every PostgreSQL integration run must use a fresh empty database or reset its disposable Compose volume first; an already-applied Stage 1 migration is not a valid Stage 2 test environment.
- Use `7bbf114` (the reviewed final Stage 1 commit) as the fixed diff base for Stage 2 scope and whitespace gates.
- All import, data-source, and import-detail endpoints require the existing `PROPULSE_ACCESS_TOKEN`. Neighborhood identity, current market data, metric history, provenance, and quality metadata remain public.
- JSON and CSV imports normalize into one command, contain 1-500 records total, and persist the collection run plus all observations atomically. A failed observation write must leave no run or sibling observations.
- Exact idempotency is database-enforced by `(data_source_id, source_ref, content_checksum)`. A replay returns the original run and does not duplicate observations.
- Store the accepted raw JSON body or CSV file bytes and a validation summary. Malformed or semantically invalid requests return an error without creating a collection run.
- API and database monetary values continue the existing product convention of `万元`, with at most two decimal places. Do not silently reinterpret existing values as yuan or cents.
- Current inventory comes only from the most recent completed `full` run. A newer `partial` run may update last-collection provenance, but it never contributes an inventory total.
- Derive price reductions by comparing stable source listing IDs with earlier observations from the same data source. Never accept or persist a trusted `priceCut` input.
- Transaction ranges use deduplicated transaction observations dated within the 90 days ending at the latest collection time.
- Freshness is `current` through 7 days, `stale` after 7 through 30 days, and `expired` after 30 days, measured from the full inventory run used by the metric. Only full, current data with at least 5 active listings and 3 recent transactions may produce a sufficient market signal. A first full run may have no derived price cuts without being low quality.
- Partial, stale, expired, missing-full-inventory, or insufficient-sample data returns a low-confidence wait/insufficient-data result with explicit warning codes.
- A newer collection run whose metric refresh is pending or failed must downgrade public market quality with `metric_refresh_pending`; an older metric must never appear to include that run.
- Runtime behavior changes use test-driven development: write and observe each focused failure before implementation. Schema rewrites, generated sqlc/OpenAPI files, and documentation are mechanical exceptions but still require explicit verification.
- Do not log bearer tokens, raw import payloads, source notes, or personal financial input. Request IDs and collection-run IDs are safe log fields.

---

## File Structure

### Files Created

- `internal/domain/neighborhood/quality.go`: coverage, freshness, warning, confidence, and recommendation eligibility rules.
- `internal/domain/neighborhood/quality_test.go`: boundary and insufficiency tests for market quality.
- `internal/application/collection/types.go`: data-source, collection-run, normalized observation, result, and detail types.
- `internal/application/collection/validation.go`: deterministic batch and record validation with row/field issues.
- `internal/application/collection/checksum.go`: exact replay checksum for accepted JSON/CSV content.
- `internal/interfaces/http/handler/errors.go`: shared structured API error and validation-detail writer.
- `internal/interfaces/http/handler/admin_data_sources.go`: protected source create/list adapters.
- `internal/interfaces/http/handler/admin_data_sources_test.go`: data-source HTTP contract tests.
- `internal/interfaces/http/handler/admin_imports_csv.go`: bounded multipart CSV parsing and normalization.
- `internal/interfaces/http/handler/admin_imports_csv_test.go`: header, row, type, and size tests.
- `internal/infrastructure/postgres/sqlmetric/repository_postgres_test.go`: PostgreSQL-backed aggregation and provenance tests.
- `internal/platform/app/e2e_market_test.go`: full Stage 2 source/import/market/history flow.

### Files Replaced Or Substantially Reworked

- `migrations/000001_initial_schema.up.sql`: replace raw records/snapshots with sources, runs, observations, and provenance-rich metrics.
- `migrations/000001_initial_schema.down.sql`: drop the Stage 2 schema in reverse dependency order.
- `internal/application/collection/imports.go`: replace `ImportManualListings` with normalized, idempotent collection-run orchestration.
- `internal/application/collection/ports.go`: expose transactional persistence, source lifecycle, run detail, and post-commit metric refresh ports.
- `internal/infrastructure/postgres/gorm/collection_repository.go`: atomic source/run/observation persistence and replay handling.
- `queries/neighborhood_metrics.sql`: latest/full-run, prior-listing, 90-day transaction, upsert, latest, and history queries.
- `internal/infrastructure/postgres/sqlmetric/repository.go`: map sqlc aggregates and metric provenance.
- `internal/application/metric/calculate_neighborhood.go`: calculate quality-aware, retry-safe metrics from committed runs.
- `internal/application/neighborhood/queries.go`: public selector, market page, and weekly metric history read models.
- `internal/interfaces/http/handler/admin_imports.go`: JSON import and batch-detail adapters.
- `internal/interfaces/http/handler/neighborhood.go`: public list, market, and history handlers.
- `api/openapi.yaml`: Stage 2 route, schema, provenance, validation, and security contract.

### Generated Files

- `internal/infrastructure/postgres/sqlc/models.go`
- `internal/infrastructure/postgres/sqlc/neighborhood_metrics.sql.go`
- `apps/web/src/lib/generated-api.d.ts`

### Files Removed

- No source file is removed. The old `/admin/api/imports` and `/api/v1/neighborhoods/{id}/metrics` routes are removed from router and OpenAPI registration when their replacements land.

---

### Task 1: Expand The Undeployed Schema For A Safe Cutover

**Files:**
- Modify: `migrations/000001_initial_schema.up.sql`
- Modify: `migrations/000001_initial_schema.down.sql`
- Modify: `migrations/embed_test.go`

**Interfaces:**
- Produces: additive `data_sources`, `collection_runs`, `listing_observations`, and `transaction_observations` tables plus nullable provenance columns on `neighborhood_metrics`.
- Preserves temporarily: `raw_collection_records`, `listing_snapshots`, and legacy metric rows so every intermediate commit remains runnable.
- Finalizes later: Task 11 removes the legacy tables and makes the new metric provenance columns mandatory in the same undeployed `000001` migration.

- [ ] **Step 1: Write the failing expansion contract**

Keep the exact two-file migration-set and stable-user assertions. Add:

```go
for _, required := range []string{
	"CREATE TABLE data_sources",
	"CREATE TABLE collection_runs",
	"CREATE TABLE listing_observations",
	"CREATE TABLE transaction_observations",
	"UNIQUE (data_source_id, source_ref, content_checksum)",
	"UNIQUE (collection_run_id, source_listing_id)",
	"UNIQUE (collection_run_id, source_record_id)",
	"metric_status TEXT NOT NULL",
	"inventory_collection_run_id UUID",
	"FOREIGN KEY (collection_run_id, neighborhood_id)",
	"ON DELETE SET NULL (inventory_collection_run_id)",
	"avg_days_on_market NUMERIC(8,2),",
	"listing_price_min NUMERIC(12,2),",
	"listing_price_max NUMERIC(12,2),",
	"transaction_price_min NUMERIC(12,2),",
	"transaction_price_max NUMERIC(12,2),",
	"idx_collection_runs_neighborhood_collected_at",
	"idx_listing_observations_source_history",
	"idx_transaction_observations_neighborhood_date",
} {
	if !strings.Contains(string(body), required) {
		t.Fatalf("expanded initial schema is missing %q", required)
	}
}
```

Do not add the final legacy-name negative assertions yet; Task 11 adds them at contraction.

- [ ] **Step 2: Run the migration test and observe the missing tables**

```bash
go test ./migrations -run TestEmbeddedMigrationSetIsSingleCoherentInitialSchema -v
```

Expected: FAIL because the four new tables and metric provenance columns are absent.

- [ ] **Step 3: Add the trusted collection tables**

Insert these tables after the existing watchlist table while retaining the two legacy collection tables:

```sql
CREATE TABLE data_sources (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 128),
  source_type TEXT NOT NULL CHECK (source_type ~ '^[a-z][a-z0-9_]{0,63}$'),
  city TEXT NOT NULL CHECK (char_length(city) BETWEEN 1 AND 128),
  notes TEXT NOT NULL DEFAULT '' CHECK (char_length(notes) <= 2048),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (name, city)
);

CREATE TABLE collection_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  data_source_id UUID NOT NULL REFERENCES data_sources(id) ON DELETE RESTRICT,
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  source_ref TEXT NOT NULL CHECK (char_length(source_ref) BETWEEN 1 AND 256),
  collected_at TIMESTAMPTZ NOT NULL,
  coverage TEXT NOT NULL CHECK (coverage IN ('full', 'partial')),
  import_format TEXT NOT NULL CHECK (import_format IN ('json', 'csv')),
  content_checksum TEXT NOT NULL CHECK (content_checksum ~ '^[0-9a-f]{64}$'),
  raw_payload BYTEA NOT NULL,
  raw_content_type TEXT NOT NULL,
  validation_summary JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'completed' CHECK (status = 'completed'),
  metric_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (metric_status IN ('pending', 'completed', 'failed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (data_source_id, source_ref, content_checksum),
  UNIQUE (id, neighborhood_id)
);

CREATE TABLE listing_observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_run_id UUID NOT NULL,
  neighborhood_id UUID NOT NULL,
  source_listing_id TEXT NOT NULL CHECK (char_length(source_listing_id) BETWEEN 1 AND 128),
  source_row INT NOT NULL CHECK (source_row >= 1),
  layout TEXT NOT NULL CHECK (char_length(layout) BETWEEN 1 AND 64),
  area_sqm NUMERIC(8,2) NOT NULL CHECK (area_sqm > 0 AND area_sqm <= 10000),
  listing_price NUMERIC(12,2) NOT NULL CHECK (listing_price > 0),
  days_on_market INT NOT NULL CHECK (days_on_market BETWEEN 0 AND 36500),
  status TEXT NOT NULL CHECK (status IN ('active', 'pending', 'withdrawn', 'sold')),
  captured_at TIMESTAMPTZ NOT NULL,
  attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
  FOREIGN KEY (collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  UNIQUE (collection_run_id, source_listing_id)
);

CREATE TABLE transaction_observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_run_id UUID NOT NULL,
  neighborhood_id UUID NOT NULL,
  source_record_id TEXT NOT NULL CHECK (char_length(source_record_id) BETWEEN 1 AND 128),
  source_row INT NOT NULL CHECK (source_row >= 1),
  layout TEXT NOT NULL CHECK (char_length(layout) BETWEEN 1 AND 64),
  area_sqm NUMERIC(8,2) NOT NULL CHECK (area_sqm > 0 AND area_sqm <= 10000),
  transaction_price NUMERIC(12,2) NOT NULL CHECK (transaction_price > 0),
  transaction_date DATE NOT NULL,
  original_listing_ref TEXT,
  captured_at TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  UNIQUE (collection_run_id, source_record_id)
);
```

- [ ] **Step 4: Expand metrics without breaking legacy inserts**

Make the existing `avg_days_on_market`, `listing_price_min`, `listing_price_max`, `transaction_price_min`, and `transaction_price_max` columns nullable in the rewritten undeployed definition. Legacy inserts still provide all five values, so this is backward-compatible with the Stage 1 GORM and sqlc callers while permitting Task 5's partial-only metric snapshots to preserve absent ranges as SQL `NULL`.

Add nullable/defaulted provenance columns to the existing `neighborhood_metrics` definition:

```sql
  collection_run_id UUID,
  inventory_collection_run_id UUID,
  source_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  listing_sample_count INT NOT NULL DEFAULT 0 CHECK (listing_sample_count >= 0),
  transaction_sample_count INT NOT NULL DEFAULT 0 CHECK (transaction_sample_count >= 0),
  listed_homes_change_pct NUMERIC(8,2),
  coverage TEXT CHECK (coverage IN ('full', 'partial')),
  freshness TEXT CHECK (freshness IN ('unknown', 'current', 'stale', 'expired')),
  quality_state TEXT CHECK (quality_state IN ('sufficient', 'low_confidence', 'insufficient_data')),
  latest_observed_at TIMESTAMPTZ,
  inventory_collected_at TIMESTAMPTZ,
  quality_warnings JSONB NOT NULL DEFAULT '[]'::jsonb,
  FOREIGN KEY (collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  FOREIGN KEY (inventory_collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id)
    ON DELETE SET NULL (inventory_collection_run_id),
  UNIQUE (collection_run_id)
```

Add the three collection indexes from Step 1 and keep the existing metric-history index. Update the down migration so metrics drop first, then new observations/runs/sources, then the retained legacy collection tables, watchlist, neighborhoods, and capacity.

- [ ] **Step 5: Verify the additive schema and unchanged application**

```bash
go test ./migrations -v
go test ./...
git diff --check
```

Expected: PASS. No Go model or generated sqlc file changes in this task; legacy inserts continue to provide non-null metric values, and the legacy application remains runnable until the cutover tasks land.

- [ ] **Step 6: Commit the schema expansion**

```bash
git add migrations
git commit -m "feat: expand trusted market data schema"
```

---

### Task 2: Add Market Quality And Recommendation Gates

**Files:**
- Create: `internal/domain/neighborhood/quality.go`
- Create: `internal/domain/neighborhood/quality_test.go`
- Modify: `internal/domain/neighborhood/signal.go`
- Modify: `internal/domain/neighborhood/signal_test.go`

**Interfaces:**
- Produces: `AssessQuality(QualityInput) QualityAssessment`.
- Produces: `SignalInput.Quality` and a deterministic insufficient-data signal.
- Consumes later: latest market, watchlist, and action-window application services.

- [ ] **Step 1: Write failing freshness and eligibility tests**

Add table-driven tests for the exact boundaries:

```go
func TestAssessQualityFreshnessBoundaries(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		age  time.Duration
		want Freshness
	}{
		{name: "seven days is current", age: 7 * 24 * time.Hour, want: FreshnessCurrent},
		{name: "after seven days is stale", age: 7*24*time.Hour + time.Second, want: FreshnessStale},
		{name: "thirty days is stale", age: 30 * 24 * time.Hour, want: FreshnessStale},
		{name: "after thirty days is expired", age: 30*24*time.Hour + time.Second, want: FreshnessExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectedAt := now.Add(-tt.age)
			got := AssessQuality(QualityInput{
				Now: now, InventoryCollectedAt: &collectedAt, LatestCoverage: CoverageFull,
				HasFullInventory: true, ListingSampleCount: 5, TransactionSampleCount: 3,
			})
			if got.Freshness != tt.want { t.Fatalf("Freshness = %q, want %q", got.Freshness, tt.want) }
		})
	}
}
```

Add `TestAssessQualityRejectsPartialCoverage`, `TestAssessQualityDowngradesPendingMetricRefresh`, `TestAssessQualityRequiresFullInventory`, `TestAssessQualityRequiresFiveListings`, and `TestAssessQualityRequiresThreeTransactions`. Missing full inventory or zero listing/transaction samples produces `insufficient_data`; every other failed threshold produces `low_confidence`. Each test must assert `CanRecommend == false` and the exact warning code.

- [ ] **Step 2: Run the quality tests and verify the missing API fails**

Run:

```bash
go test ./internal/domain/neighborhood -run 'TestAssessQuality' -v
```

Expected: FAIL because the quality types and function do not exist.

- [ ] **Step 3: Implement deterministic quality assessment**

Define:

```go
type Coverage string
const (
	CoverageUnknown Coverage = "unknown"
	CoverageFull Coverage = "full"
	CoveragePartial Coverage = "partial"
)

type Freshness string
const (
	FreshnessUnknown Freshness = "unknown"
	FreshnessCurrent Freshness = "current"
	FreshnessStale Freshness = "stale"
	FreshnessExpired Freshness = "expired"
)

type MarketQualityState string
const (
	MarketQualitySufficient MarketQualityState = "sufficient"
	MarketQualityLowConfidence MarketQualityState = "low_confidence"
	MarketQualityInsufficientData MarketQualityState = "insufficient_data"
)

type QualityWarning string
const (
	WarningPartialCoverage QualityWarning = "partial_coverage"
	WarningNoFullInventory QualityWarning = "no_full_inventory"
	WarningStaleData QualityWarning = "stale_data"
	WarningExpiredData QualityWarning = "expired_data"
	WarningInsufficientListings QualityWarning = "insufficient_listing_samples"
	WarningInsufficientTransactions QualityWarning = "insufficient_transaction_samples"
	WarningMetricRefreshPending QualityWarning = "metric_refresh_pending"
)

type QualityInput struct {
	Now time.Time
	InventoryCollectedAt *time.Time
	LatestCoverage Coverage
	HasFullInventory bool
	ListingSampleCount int
	TransactionSampleCount int
	HasNewerUncalculatedRun bool
}

type QualityAssessment struct {
	Coverage Coverage `json:"coverage"`
	Freshness Freshness `json:"freshness"`
	State MarketQualityState `json:"state"`
	CanRecommend bool `json:"canRecommend"`
	Warnings []QualityWarning `json:"warnings"`
}
```

`CoverageUnknown` is a read-model state only; import validation accepts only `full` or `partial`. `AssessQuality` uses `FreshnessUnknown` when no full inventory timestamp exists. It appends warnings in this stable order: partial coverage, pending metric refresh, missing full inventory, expired or stale data, insufficient listings, insufficient transactions. It returns `sufficient` and `CanRecommend=true` only when the warning slice is empty. It returns `insufficient_data` when full inventory, active listings, or recent transactions are absent; all other warning combinations return `low_confidence`.

- [ ] **Step 4: Write a failing signal trust test**

Add:

```go
func TestEvaluateSignalWaitsWhenQualityCannotRecommend(t *testing.T) {
	result := EvaluateSignal(SignalInput{
		ListedHomes: 42, PriceCutHomes: 11,
		TransactionMomentum: TransactionMomentumWeak,
		Quality: QualityAssessment{
			Coverage: CoveragePartial,
			Freshness: FreshnessCurrent,
			State: MarketQualityLowConfidence,
			CanRecommend: false,
			Warnings: []QualityWarning{WarningPartialCoverage},
		},
	})
	if result.Status != NeighborhoodStatusInsufficientData { t.Fatalf("Status = %q", result.Status) }
	if result.QualityState != MarketQualityLowConfidence { t.Fatalf("QualityState = %q", result.QualityState) }
	if result.SupplyPressure != SupplyPressureUnknown { t.Fatalf("SupplyPressure = %q", result.SupplyPressure) }
	if result.TargetLayoutScarcity != ScarcityUnknown { t.Fatalf("TargetLayoutScarcity = %q", result.TargetLayoutScarcity) }
	if result.NextAction != "等待补充完整且新鲜的挂牌与成交样本，再判断看房或议价时机。" {
		t.Fatalf("NextAction = %q", result.NextAction)
	}
}
```

- [ ] **Step 5: Gate the existing signal evaluator without breaking the additive phase**

Add `NeighborhoodStatusInsufficientData = "数据不足"`, `SupplyPressureUnknown = "unknown"`, and `ScarcityUnknown = "unknown"`. Add `Quality QualityAssessment` to `SignalInput`, and add `QualityState MarketQualityState` plus `Warnings []QualityWarning` to `SignalResult`. In this additive task, return the exact wait result with unknown supply/scarcity before any price/supply calculation only when `input.Quality.State != "" && !input.Quality.CanRecommend`; this preserves the current legacy metric callers, which cannot yet populate provenance-aware quality. Existing trusted signal tests must pass an explicit `sufficient` quality value.

The wait result copies `Quality.State` and `Quality.Warnings`, sets `Reasons` to `[]string{"市场数据覆盖、样本量或新鲜度不足，不能据此给出买入或议价结论。"}`, and uses the exact next action asserted above. It never evaluates price gap, price-cut share, supply pressure, or target-layout scarcity from missing ranges.

Task 5 changes this temporary compatibility condition to fail closed: an empty quality state becomes `insufficient_data`. Add the final `TestEvaluateSignalFailsClosedWithoutQualityAfterTrustedMetricCutover` there after every metric reader populates quality explicitly.

- [ ] **Step 6: Verify the additive gate and commit**

```bash
go test ./internal/domain/neighborhood -v
go test ./...
git add internal/domain/neighborhood
git commit -m "feat: gate market signals on data quality"
```

Expected: all neighborhood-domain tests PASS, including freshness boundaries and prevention of a bargain signal from partial data.

---

### Task 3: Define The Normalized Collection Contract

**Files:**
- Create: `internal/application/collection/types.go`
- Create: `internal/application/collection/validation.go`
- Create: `internal/application/collection/checksum.go`
- Create: `internal/application/collection/validation_test.go`
- Create: `internal/application/collection/checksum_test.go`

**Interfaces:**
- Produces: normalized source/run/observation types, `validateAndNormalize`, `contentChecksum`, and structured validation issues.
- Preserves: the current `ImportManualListings` service and repository interface unchanged, so the production composition root keeps compiling.
- Consumes later: Task 4 adds persistence and application orchestration; Task 9 adds post-commit metric refresh.

- [ ] **Step 1: Write failing normalized-command tests without changing the current service**

Write these focused tests before deleting the old implementation:

```text
TestValidateAndNormalizeSplitsListingAndTransaction
TestValidateAndNormalizeRejectsInvalidRecordsWithAllIssues
TestValidateAndNormalizeRejectsMoreThanFiveHundredRecords
TestValidateAndNormalizeRejectsDuplicateSourceRecordIDs
TestValidateAndNormalizeTrimsSourceAndRecordFields
TestContentChecksumIsStableForExactReplay
TestContentChecksumChangesWithRawBytesOrImportMetadata
```

The first test must supply one listing and one transaction, assert separate normalized slices, and assert no normalized type contains a caller-supplied `PriceCut` field. These tests do not call a repository.

- [ ] **Step 2: Run the focused tests and verify the new contract is absent**

Run:

```bash
go test ./internal/application/collection -run 'Test(ValidateAndNormalize|ContentChecksum)' -v
```

Expected: FAIL because the normalized types, validator, and checksum do not exist.

- [ ] **Step 3: Define the normalized types**

Use these stable type names across later tasks:

```go
type ImportFormat string
const (
	ImportFormatJSON ImportFormat = "json"
	ImportFormatCSV ImportFormat = "csv"
)

type RecordType string
const (
	RecordTypeListing RecordType = "listing"
	RecordTypeTransaction RecordType = "transaction"
)

type ListingStatus string
const (
	ListingStatusActive ListingStatus = "active"
	ListingStatusPending ListingStatus = "pending"
	ListingStatusWithdrawn ListingStatus = "withdrawn"
	ListingStatusSold ListingStatus = "sold"
)

type ObservationInput struct {
	Row int
	RecordType RecordType
	SourceRecordID string
	Layout string
	AreaSQM float64
	ListingPrice *float64
	TransactionPrice *float64
	TransactionDate *time.Time
	DaysOnMarket *int
	Status *ListingStatus
	OriginalListingRef *string
	Attributes map[string]string
}

type ImportCollectionRunCommand struct {
	DataSourceID string
	NeighborhoodID string
	SourceRef string
	CollectedAt time.Time
	Coverage domainneighborhood.Coverage
	Format ImportFormat
	RawPayload []byte
	RawContentType string
	Records []ObservationInput
}

type ValidationIssue struct {
	Row *int `json:"row,omitempty"`
	Field string `json:"field"`
	Code string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct { Issues []ValidationIssue }
func (e *ValidationError) Error() string { return "one or more import fields are invalid" }

var ErrDataSourceNotFound = errors.New("data_source_not_found")
var ErrCollectionRunNotFound = errors.New("collection_run_not_found")

type MetricStatus string
const (
	MetricStatusPending MetricStatus = "pending"
	MetricStatusCompleted MetricStatus = "completed"
	MetricStatusFailed MetricStatus = "failed"
)

type CollectionRunStatus string
const (
	CollectionRunStatusCompleted CollectionRunStatus = "completed"
)

type DataSource struct {
	ID string
	Name string
	SourceType string
	City string
	Notes string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateDataSourceCommand struct {
	Name string
	SourceType string
	City string
	Notes string
}

type CollectionRun struct {
	ID string
	DataSourceID string
	NeighborhoodID string
	SourceRef string
	CollectedAt time.Time
	Coverage domainneighborhood.Coverage
	Format ImportFormat
	ContentChecksum string
	RawPayload []byte
	RawContentType string
	ValidationSummary ValidationSummary
	Status CollectionRunStatus
	MetricStatus MetricStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListingObservation struct {
	ID string
	CollectionRunID string
	NeighborhoodID string
	SourceListingID string
	SourceRow int
	Layout string
	AreaSQM float64
	ListingPrice float64
	DaysOnMarket int
	Status ListingStatus
	CapturedAt time.Time
	Attributes map[string]string
}

type TransactionObservation struct {
	ID string
	CollectionRunID string
	NeighborhoodID string
	SourceRecordID string
	SourceRow int
	Layout string
	AreaSQM float64
	TransactionPrice float64
	TransactionDate time.Time
	OriginalListingRef *string
	CapturedAt time.Time
}

type ValidationSummary struct {
	RecordCount int `json:"recordCount"`
	ListingCount int `json:"listingCount"`
	TransactionCount int `json:"transactionCount"`
	Issues []ValidationIssue `json:"issues"`
}

type NormalizedImport struct {
	DataSourceID string
	NeighborhoodID string
	SourceRef string
	CollectedAt time.Time
	Coverage domainneighborhood.Coverage
	Format ImportFormat
	RawPayload []byte
	RawContentType string
	Listings []ListingObservation
	Transactions []TransactionObservation
	ValidationSummary ValidationSummary
}

type ImportBatch struct {
	Run CollectionRun
	Listings []ListingObservation
	Transactions []TransactionObservation
}

type SaveImportResult struct {
	Run CollectionRun
	Created bool
}

type CollectionRunDetail struct {
	Run CollectionRun
	Source DataSource
	Listings []ListingObservation
	Transactions []TransactionObservation
}

type GetCollectionRunQuery struct { ID string }

type ImportCollectionRunResult struct {
	Run CollectionRun
	ListingCount int
	TransactionCount int
	IdempotentReplay bool
	MetricRefreshStatus MetricStatus
}

func validateAndNormalize(command ImportCollectionRunCommand, now time.Time) (NormalizedImport, []ValidationIssue)
func (normalized NormalizedImport) NewBatch(runID string, newID func() string) ImportBatch
```

`NewBatch` copies raw bytes, creates a `completed` collection run with the generated ID, calculates `ContentChecksum` from the normalized command metadata and exact raw bytes, sets `MetricStatusPending`, and assigns generated observation IDs plus the run ID, command neighborhood ID, and collection timestamp to every observation. It is called only after `validateAndNormalize` returns no issues, so invalid commands generate no IDs. Do not modify the existing service or repository interfaces in this task.

- [ ] **Step 4: Implement exact normalization and validation**

`validateAndNormalize(command, now)` must:

- trim IDs, source reference, layout, status, and original listing reference;
- require parseable UUID data-source and neighborhood IDs;
- require a UTC `CollectedAt` no more than five minutes in the future;
- require `full` or `partial`, `json` or `csv`, and 1-500 combined records;
- require raw payload length between 1 byte and 2 MiB and a nonblank raw content type of at most 255 bytes, so future non-HTTP producers cannot bypass adapter limits;
- require `SourceRef` length 1-256 and `SourceRecordID` length 1-128;
- reject duplicate `(recordType, sourceRecordID)` pairs in one batch;
- require a trimmed layout of length 1-64 and an optional original listing reference of at most 128 characters;
- require finite `AreaSQM` in `(0, 10000]` and finite positive prices with no more than two decimal places;
- require listing price, days on market `0..36500`, and one allowed listing status for listing rows while rejecting transaction-only fields;
- require transaction price and a transaction date no later than the UTC collection date for transaction rows while rejecting listing-only fields;
- limit JSON attributes to 20 string pairs, key length 1-64, and value length at most 512;
- collect all issues in input order instead of returning only the first issue.

JSON record rows are one-based indexes into `records`; CSV adapters set physical row numbers beginning at 2. Batch issues have `Row=nil`.

- [ ] **Step 5: Implement the exact replay checksum**

Hash this envelope with SHA-256 and lower-case hex:

```go
func contentChecksum(command ImportCollectionRunCommand) string {
	h := sha256.New()
	for _, value := range []string{
		string(command.Format),
		command.DataSourceID,
		command.NeighborhoodID,
		command.SourceRef,
		command.CollectedAt.UTC().Format(time.RFC3339Nano),
		string(command.Coverage),
	} {
		_, _ = io.WriteString(h, value)
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write(command.RawPayload)
	return hex.EncodeToString(h.Sum(nil))
}
```

The database identity also enforces data-source ID and source reference. Including them in the checksum envelope makes the checksum self-describing; exact raw-byte or import-metadata changes intentionally produce a new run, and JSON/CSV are not semantically deduplicated across formats.

- [ ] **Step 6: Verify the additive contract**

Run:

```bash
go test ./internal/application/collection -v
go test ./...
git diff --check
```

Expected: all collection and repository callers still compile; the new pure tests PASS; legacy names remain only until the application cutover.

- [ ] **Step 7: Commit the normalized contract**

```bash
git add internal/application/collection/types.go internal/application/collection/validation.go internal/application/collection/checksum.go internal/application/collection/validation_test.go internal/application/collection/checksum_test.go
git commit -m "feat: define normalized collection contract"
```

---

### Task 4: Persist Collection Runs And Observation Kinds Atomically

**Files:**
- Modify: `internal/infrastructure/postgres/gorm/collection_repository.go`
- Modify: `internal/infrastructure/postgres/gorm/collection_repository_test.go`
- Modify: `internal/interfaces/http/router/collection_memory.go`
- Modify: `internal/interfaces/http/router/router_test.go`
- Modify: `internal/infrastructure/postgres/gorm/models.go`
- Modify: `internal/application/collection/ports.go`
- Modify: `internal/interfaces/http/router/collection_memory.go`
- Modify: `internal/interfaces/http/router/neighborhood_memory.go`

**Interfaces:**
- Consumes: `collection.ImportBatch`, `SaveImportResult`, `CollectionRunDetail`, and normalized observation types from Task 3.
- Produces: a GORM implementation of the collection repository and a fallback implementation that shares the router's neighborhood store.
- Guarantees: one collection run and all of its normalized observations commit or roll back together.

- [ ] **Step 0: Add the new repository contract beside the legacy one**

Do not mutate `Repository` yet because it is still used by `ImportManualListings`. Add a second interface in `ports.go`:

```go
type TrustedRepository interface {
	CreateDataSource(context.Context, DataSource) (DataSource, error)
	ListDataSources(context.Context) ([]DataSource, error)
	DataSourceExists(context.Context, string) (bool, error)
	NeighborhoodExists(context.Context, string) (bool, error)
	SaveCollectionRun(context.Context, ImportBatch) (SaveImportResult, error)
	GetCollectionRun(context.Context, string) (CollectionRunDetail, error)
	UpdateMetricStatus(context.Context, string, MetricStatus) error
}
```

The GORM and in-memory repositories implement both interfaces during the cutover. The old service continues to use `Repository` until the end of Step 7.

- [ ] **Step 1: Write failing PostgreSQL repository tests**

Replace the skipped legacy snapshot test with these tests. Every test calls `migraterunner.Run(ctx, databaseURL, "up")`, creates a source plus neighborhood, and skips only when `PROPULSE_TEST_DATABASE_URL` is absent:

```text
TestCollectionRepositorySaveCollectionRunPersistsRunAndBothObservationTypes
TestCollectionRepositorySaveCollectionRunReturnsExistingRunForDuplicateIdentity
TestCollectionRepositorySaveCollectionRunRollsBackWhenChildInsertFails
TestCollectionRepositorySaveCollectionRunAllowsTransactionOnlyBatch
TestCollectionRepositoryGetCollectionRunReturnsTraceability
TestCollectionRepositoryConcurrentDuplicateImportsCreateOneRun
```

The first test must assert raw bytes, `raw_content_type`, checksum, source-row values, empty validation errors, and both observation kinds. The duplicate test must assert `Created=false`, same run ID, and exactly one row in each table. The concurrency test launches two saves with the same identity and asserts one created run plus one replay, not two runs.

- [ ] **Step 2: Run the repository tests and verify the old persistence API fails**

Run:

```bash
PROPULSE_TEST_DATABASE_URL="$PROPULSE_TEST_DATABASE_URL" \
go test ./internal/infrastructure/postgres/gorm -run TestCollectionRepository -v
```

Expected: FAIL to compile because `TrustedRepository`, `SaveCollectionRun`, and the new observation models do not exist yet; the old `SaveImport` remains intentionally available during this additive task. Before PostgreSQL integration tests, use a fresh database created from the rewritten undeployed migration (for Compose, `docker compose down -v` followed by `docker compose up -d postgres`) so an already-applied legacy `000001` schema cannot hide missing tables. If the variable is unset, first run the same command with a local PostgreSQL URL or record an explicit skip; do not treat skipped integration tests as proving transaction behavior.

- [ ] **Step 3: Implement `SaveCollectionRun` transactionally**

Implement `SaveCollectionRun` as one GORM transaction:

```go
func (r *CollectionRepository) SaveCollectionRun(ctx context.Context, batch appcollection.ImportBatch) (appcollection.SaveImportResult, error) {
	var result appcollection.SaveImportResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run := collectionRunModel(batch.Run)
		create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&run)
		if create.Error != nil { return create.Error }
		if create.RowsAffected == 0 {
			var existing CollectionRunModel
			if err := tx.Where("data_source_id = ? AND source_ref = ? AND content_checksum = ?", run.DataSourceID, run.SourceRef, run.ContentChecksum).First(&existing).Error; err != nil { return err }
			result = appcollection.SaveImportResult{Run: collectionRunFromModel(existing), Created: false}
			return nil
		}

		listings := listingObservationModels(batch.Run, batch.Listings)
		transactions := transactionObservationModels(batch.Run, batch.Transactions)
		if len(listings) > 0 {
			if err := tx.Create(&listings).Error; err != nil { return err }
		}
		if len(transactions) > 0 {
			if err := tx.Create(&transactions).Error; err != nil { return err }
		}
		result = appcollection.SaveImportResult{Run: collectionRunFromModel(run), Created: true}
		return nil
	})
	return result, err
}
```

Child model conversion must always take collection-run and neighborhood IDs from `batch.Run`, never values duplicated in an observation input. Use `clause.OnConflict{DoNothing:true}` only for the run identity; a duplicate child remains a transaction failure rather than silently hiding bad input.

- [ ] **Step 4: Implement source, detail, and metric-status persistence**

`CreateDataSource` persists normalized source fields. A duplicate `(name, city)` is idempotent: use `ON CONFLICT (name, city) DO NOTHING`, then select and return the existing row. `ListDataSources` orders by `city ASC, name ASC, id ASC`. `GetCollectionRun` loads source metadata, run metadata, listings ordered by source row, transactions ordered by source row, and raw bytes without decoding them. Map `gorm.ErrRecordNotFound` to `collection.ErrCollectionRunNotFound`.

Add the `DataSourceExists` query beside `NeighborhoodExists`, not as an implicit create-on-import behavior.

`UpdateMetricStatus` must update only `collection_runs.metric_status` and `updated_at` for a known run, returning `ErrCollectionRunNotFound` when no row is affected.

- [ ] **Step 5: Make fallback routing state coherent**

Replace the disconnected `newInMemoryCollectionRepository()` construction with an adapter that holds the same `*inMemoryNeighborhoodRepository` instance created by `router.New`. Its `NeighborhoodExists` delegates to that store. Put source summaries, collection-run state, and run detail maps in one shared in-memory market-state object with its own `sync.RWMutex`, held by both repositories. `SaveCollectionRun` publishes its run/source state to that shared object before returning; `LatestCollectionRunState`, source-summary lookup, and later public market reads use the same object. Apply the same idempotency key behavior so router tests can create a neighborhood, import it, and observe its pending provenance in one engine.

- [ ] **Step 6: Verify transaction and fallback behavior**

Run:

```bash
go test ./internal/infrastructure/postgres/gorm ./internal/interfaces/http/router -v
go test -race ./internal/infrastructure/postgres/gorm ./internal/interfaces/http/router
go test ./...
git diff --check
```

Expected: unit and router fallback tests PASS; PostgreSQL tests pass when configured or explicitly skip only because the database environment is absent.

- [ ] **Step 7: Commit atomic persistence**

```bash
git add internal/application/collection/ports.go internal/infrastructure/postgres/gorm internal/interfaces/http/router
git commit -m "feat: persist collection runs atomically"
```

---

### Task 5: Derive Provenance-Aware Market Metrics

**Files:**
- Modify: `queries/neighborhood_metrics.sql`
- Regenerate: `internal/infrastructure/postgres/sqlc/models.go`
- Regenerate: `internal/infrastructure/postgres/sqlc/neighborhood_metrics.sql.go`
- Modify: `internal/infrastructure/postgres/sqlmetric/repository.go`
- Modify: `internal/infrastructure/postgres/sqlmetric/repository_test.go`
- Create: `internal/infrastructure/postgres/sqlmetric/repository_postgres_test.go`
- Modify: `internal/application/metric/ports.go`
- Modify: `internal/application/metric/calculate_neighborhood.go`
- Modify: `internal/application/metric/calculate_neighborhood_test.go`
- Modify: `internal/application/neighborhood/ports.go`
- Modify: `internal/application/neighborhood/queries.go`
- Modify: `internal/application/neighborhood/service_test.go`
- Modify: `internal/domain/neighborhood/signal.go`
- Modify: `internal/domain/neighborhood/signal_test.go`
- Modify: `internal/infrastructure/postgres/gorm/neighborhood_repository.go`
- Modify: `internal/infrastructure/postgres/gorm/neighborhood_repository_test.go`

**Interfaces:**
- Consumes: completed collection runs and quality assessment from Tasks 1-4.
- Produces: one idempotent trusted metric snapshot per collection run, public metric provenance, and chronological history.
- Preserves temporarily: `AggregateListingSnapshots`, `InsertNeighborhoodMetric`, and `CalculateNeighborhood(ctx, neighborhoodID)` so the Stage 1 import service and scheduler remain runnable through Tasks 6-8.
- Finalizes later: Task 9 moves every production trigger to `CalculateCollectionRun`; Task 11 deletes the legacy metric queries, generated types, models, methods, and tests together with the legacy tables.

- [ ] **Step 1: Write failing metric application tests**

Add the new run-aware tests beside the legacy aggregate tests. New tests exercise `CalculateCollectionRun`. Preserve the legacy `CalculateNeighborhood(ctx, neighborhoodID)` behavior until Task 11, but update its fixtures, pointer assertions, implementation assignments, and `InsertNeighborhoodMetric` mapper when `MetricSnapshot` changes nullable values from `float64` to `*float64`; otherwise the additive Task 5 commit will not compile.

```text
TestCalculateCollectionRunUsesLatestRunForProvenance
TestCalculateCollectionRunUsesLatestFullRunForInventory
TestCalculateCollectionRunDerivesPriceCutsFromPriorListingObservation
TestCalculateCollectionRunUsesOnlyTransactionsWithinNinetyDays
TestCalculateCollectionRunDoesNotCompareListingIDsAcrossSources
TestCalculateCollectionRunUsesTriggerRunAsOfTime
TestCalculateCollectionRunRejectsRunFromAnotherNeighborhood
TestCalculateCollectionRunStoresLowConfidenceForPartialOrInsufficientData
TestCalculateCollectionRunUpsertsOneMetricPerCollectionRun
TestCalculateCollectionRunMarksResolvedRunMetricCompleted
TestEvaluateSignalFailsClosedWithoutQualityAfterTrustedMetricCutover
```

The full-run test must model a newer partial run and an older full run. Assert the metric's `CollectionRunID` is the newer run but `InventoryCollectionRunID` is the older full run, proving a partial import cannot claim inventory total.

- [ ] **Step 2: Run the metric tests and verify the legacy aggregate fails**

Run:

```bash
go test ./internal/application/metric -v
```

Expected: FAIL because the trusted calculation command, aggregate, provenance fields, and explicit quality values do not exist yet. The legacy aggregate remains available by design.

- [ ] **Step 3: Add trusted sqlc query definitions beside the legacy definitions**

Define queries with these responsibilities, using explicit names so generated methods stay stable:

```sql
-- name: GetCompletedCollectionRun :one
SELECT id, data_source_id, neighborhood_id, collected_at, coverage, metric_status
FROM collection_runs
WHERE id = $1 AND status = 'completed';

-- name: LatestCompletedCollectionRun :one
SELECT id, data_source_id, neighborhood_id, collected_at, coverage, metric_status
FROM collection_runs
WHERE neighborhood_id = $1 AND status = 'completed'
ORDER BY collected_at DESC, id DESC
LIMIT 1;

-- name: LatestCompletedFullCollectionRun :one
SELECT id, data_source_id, neighborhood_id, collected_at, coverage, metric_status
FROM collection_runs
WHERE neighborhood_id = sqlc.arg(neighborhood_id)
  AND status = 'completed'
  AND coverage = 'full'
  AND (collected_at, id) <= (sqlc.arg(trigger_collected_at), sqlc.arg(trigger_run_id))
ORDER BY collected_at DESC, id DESC
LIMIT 1;

-- name: MarkCollectionRunMetricCompleted :one
UPDATE collection_runs
SET metric_status = 'completed', updated_at = now()
WHERE id = $1 AND status = 'completed'
RETURNING id;
```

`AggregateListingInventory` must read only active listings from the latest full run at or before the supplied trigger run. It returns listing count, average days, listing price range, and target-layout supply. `CountDerivedPriceCuts` compares each active listing in that inventory run to the most recent earlier successful-run row with matching `(data_source_id, neighborhood_id, source_listing_id)` and returns the derived price-cut count. `AggregateRecentTransactions` selects one latest transaction per `(data_source_id, source_record_id)`, bounds `transaction_date` to `[trigger_collected_at - interval '90 days', trigger_collected_at]`, and returns sample count, range, last-30-day count, and preceding-60-day count.

Also return the previous full inventory count and whether a newer partial trigger was excluded. When there is no earlier full inventory, persist `ListedHomesChangePct=nil` and map it to zero only for the legacy numerical signal formula; do not add a quality warning or downgrade an otherwise sufficient first full run. If no full run exists at or before the trigger, return an aggregate with nullable ranges and zero counts rather than a not-found error.

`UpsertNeighborhoodMetric` inserts all provenance and quality fields with `ON CONFLICT (collection_run_id) DO UPDATE`, making recalculation retry-safe. `MarkCollectionRunMetricCompleted` updates the resolved run's `metric_status` to `completed` only after a successful upsert and returns the run ID, so an asynchronous scheduler repair clears a pending/failed status even when its command began with an empty ID. `LatestNeighborhoodMetric` joins `collection_runs` and returns the metric attached to the latest completed trigger run ordered by `collection_runs.collected_at DESC, collection_runs.id DESC`, never merely the row with the newest `calculated_at`. `ListNeighborhoodMetricHistory` filters by trigger `collected_at >= $2` and returns ascending `collection_runs.collected_at, collection_runs.id`, one row per trigger run.

For `CalculateCollectionRun`, a non-empty `CollectionRunID` must first resolve through `GetCompletedCollectionRun`; require its `neighborhood_id` to equal `command.NeighborhoodID` and return `ErrCollectionRunNeighborhoodMismatch` before aggregation on mismatch. Aggregate only at or before that exact trigger and use the resolved run's neighborhood ID for the snapshot. An empty ID is a repair request and resolves `LatestCompletedCollectionRun` for the command neighborhood. A late repair for an older trigger may upsert its historical row, but it cannot displace the public latest read because that read orders by trigger collection time. Implement both run-resolution repository methods by mapping the Task 5 sqlc rows to `CompletedCollectionRun`, and update every metric fake to implement them.

- [ ] **Step 4: Regenerate sqlc and write failing repository mapping tests**

Run:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go test ./internal/infrastructure/postgres/sqlmetric -v
```

Expected: tests fail until `sqlmetric.Repository` maps nullable ranges, source IDs, coverage, freshness, warnings, and new query parameters.

- [ ] **Step 5: Implement metric repository and application types**

Extend `metric.MetricSnapshot` exactly:

```go
type MetricSnapshot struct {
	ID string
	NeighborhoodID string
	CollectionRunID string
	InventoryCollectionRunID *string
	SourceIDs []string
	LatestObservedAt time.Time
	ListedHomes int
	PriceCutHomes int
	AvgDaysOnMarket *float64
	ListingPriceMin *float64
	ListingPriceMax *float64
	TransactionPriceMin *float64
	TransactionPriceMax *float64
	TransactionMomentum domainneighborhood.TransactionMomentum
	TargetLayoutSupply int
	ListingSampleCount int
	TransactionSampleCount int
	Coverage domainneighborhood.Coverage
	Freshness domainneighborhood.Freshness
	InventoryCollectedAt *time.Time
	ListedHomesChangePct *float64
	QualityWarnings []domainneighborhood.QualityWarning
	QualityState domainneighborhood.MarketQualityState
	CalculatedAt time.Time
}
```

Define the exact command and repository boundary:

```go
type CalculateCollectionRunCommand struct {
	NeighborhoodID string
	CollectionRunID string
}

type AggregateMarketParams struct {
	NeighborhoodID string
	TriggerRunID string
	TargetLayout string
}

type CompletedCollectionRun struct {
	ID string
	DataSourceID string
	NeighborhoodID string
	CollectedAt time.Time
	Coverage domainneighborhood.Coverage
}

type MarketAggregate struct {
	CollectionRunID string
	InventoryCollectionRunID *string
	SourceIDs []string
	LatestObservedAt time.Time
	InventoryCollectedAt *time.Time
	Coverage domainneighborhood.Coverage
	ListedHomes int
	PriceCutHomes int
	AvgDaysOnMarket *float64
	ListingPriceMin *float64
	ListingPriceMax *float64
	TransactionPriceMin *float64
	TransactionPriceMax *float64
	TargetLayoutSupply int
	ListingSampleCount int
	TransactionSampleCount int
	LastThirtyDayTransactionCount int
	PrecedingSixtyDayTransactionCount int
	ListedHomesChangePct *float64
}

type Repository interface {
	GetNeighborhood(context.Context, string) (Neighborhood, error)
	GetCompletedCollectionRun(context.Context, string) (CompletedCollectionRun, error)
	LatestCompletedCollectionRun(context.Context, string) (CompletedCollectionRun, error)
	// Legacy methods remain until Task 11.
	AggregateListingSnapshots(context.Context, string, string) (ListingSnapshotAggregate, error)
	InsertNeighborhoodMetric(context.Context, MetricSnapshot) (MetricSnapshot, error)
	AggregateMarketObservations(context.Context, AggregateMarketParams) (MarketAggregate, error)
	UpsertNeighborhoodMetric(context.Context, MetricSnapshot) (MetricSnapshot, error)
	MarkCollectionRunMetricCompleted(context.Context, string) error
}

func NewService(repo Repository) *Service
func NewServiceWithClock(repo Repository, now func() time.Time) *Service
func (s *Service) CalculateNeighborhood(ctx context.Context, neighborhoodID string) error
func (s *Service) CalculateCollectionRun(ctx context.Context, command CalculateCollectionRunCommand) error
```

A non-empty collection-run ID is mandatory for import-triggered calculation. An empty ID means a scheduler repair and resolves the latest completed run. Compute aggregate values without substituting zero for missing ranges, call `AssessQuality` with the full inventory timestamp and the injected clock, upsert one metric attached to the trigger run, then call `MarkCollectionRunMetricCompleted` with the resolved run ID. If marking completion fails, return that error so a caller can retain/retry a non-completed status. Update every neighborhood metric mapper and `evaluateMetric` to carry the explicit quality assessment. At this point change the Task 2 compatibility gate so an empty quality state is insufficient data; this is safe because trusted metrics populate it and legacy seed/metric rows are deliberately downgraded until Task 11 removes them.

Declare `ErrCollectionRunNotFound = errors.New("collection_run_not_found")` and `ErrCollectionRunNeighborhoodMismatch = errors.New("collection_run_neighborhood_mismatch")` in the `metric` package for calculation resolution. Keep the same-named `collection.ErrCollectionRunNotFound` separate for import-detail reads so `metric` does not import `collection` and create a cycle. Handlers use `errors.Is` to map `collection.ErrDataSourceNotFound`, `collection.ErrNeighborhoodNotFound`, and `collection.ErrCollectionRunNotFound` to the exact 404 codes in Task 7; they never expose metric-internal mismatch errors because queue payloads are not HTTP input.

Stop deriving transaction momentum from listing count and price cuts. Return `unknown` below three transaction samples. Otherwise compare the final 30-day count with half of the preceding 60-day count: more than 120% is `strong`, less than 80% is `weak`, and the remaining cases are `stable`.

Add `TransactionMomentumUnknown TransactionMomentum = "unknown"` in `signal.go` and update every metric mapper/fixture to use it when there are fewer than three eligible transactions. Mirror every field of the `metric.MetricSnapshot` contract in `neighborhood.MetricSnapshot`; `evaluateMetric` reconstructs `QualityAssessment` from coverage, freshness, state, and warnings, with `CanRecommend` true only for `MarketQualitySufficient`.

- [ ] **Step 6: Add PostgreSQL-backed behavior tests**

Implement the following tests using migrations and real PostgreSQL, not SQL text inspection:

```text
TestRepositoryAggregateCollectionRunUsesOnlyItsFullInventory
TestRepositoryAggregateCollectionRunDerivesPriceCutsFromPriorObservations
TestRepositoryAggregateCollectionRunDoesNotCompareIDsAcrossSources
TestRepositoryAggregateCollectionRunUsesTransactionsWithinNinetyDays
TestRepositoryUpsertNeighborhoodMetricPersistsProvenance
TestRepositoryLatestMetricMapsProvenance
TestRepositoryMetricHistoryOrdersSnapshotsChronologically
```

The price-cut test must insert two runs from one source with the same listing ID and lower later price; assert the derived count changes even though no input includes `priceCut`.

- [ ] **Step 7: Verify metrics and commit**

```bash
go test ./internal/application/metric ./internal/application/neighborhood ./internal/domain/neighborhood ./internal/infrastructure/postgres/sqlmetric ./internal/infrastructure/postgres/gorm -v
go test -race ./internal/application/metric ./internal/application/neighborhood ./internal/domain/neighborhood ./internal/infrastructure/postgres/sqlmetric
go test ./...
git diff --check
git add queries internal/application/metric internal/application/neighborhood internal/domain/neighborhood internal/infrastructure/postgres/sqlc internal/infrastructure/postgres/sqlmetric internal/infrastructure/postgres/gorm
git commit -m "feat: calculate traceable market metrics"
```

Expected: new unit tests PASS; PostgreSQL behavior tests pass when configured or explicitly skip only if the environment is unavailable. The legacy query/method surface remains intentionally until Task 11, because Tasks 6-8 still need every intermediate commit to compile against the Stage 1 import and scheduler paths.

---

### Task 6: Expose Public Selector, Market, And History Read Models

**Files:**
- Modify: `internal/application/neighborhood/ports.go`
- Modify: `internal/application/neighborhood/commands.go`
- Modify: `internal/application/neighborhood/queries.go`
- Modify: `internal/application/neighborhood/service_test.go`
- Modify: `internal/infrastructure/postgres/gorm/neighborhood_repository.go`
- Modify: `internal/infrastructure/postgres/gorm/neighborhood_repository_test.go`
- Modify: `internal/interfaces/http/handler/neighborhood.go`
- Modify: `internal/interfaces/http/handler/neighborhood_test.go`
- Modify: `internal/interfaces/http/router/neighborhood_memory.go`
- Modify: `internal/interfaces/http/router/router.go`
- Modify: `internal/interfaces/http/router/router_test.go`
- Modify: `internal/platform/app/app.go`
- Modify: `internal/platform/app/app_test.go`

**Interfaces:**
- Produces public `GET /api/v1/neighborhoods`, `GET /api/v1/neighborhoods/{id}/market`, and `GET /api/v1/neighborhoods/{id}/metrics/history?weeks=8`.
- Replaces the public legacy `GET /api/v1/neighborhoods/{id}/metrics` route.
- Consumes provenance metric snapshots and dynamically re-assesses freshness at read time.

- [ ] **Step 1: Write failing application and handler tests**

Add:

```text
TestListNeighborhoodsOrdersSelectorRows
TestGetMarketReassessesFreshnessAtReadTime
TestGetMarketDowngradesAnOlderMetricWhenNewerRunIsPending
TestGetMarketUsesOneEffectiveQualityForResponseAndSignal
TestGetMarketReturnsInsufficientDataWhenNoMetricExists
TestGetMarketReturnsPartialSourceProvenanceWithoutFullMetric
TestGetMetricHistoryBoundsWeeksToOneThroughFiftyTwo
TestLatestMetricReassessesQualityForDecisionConsumers
TestListWatchlistReassessesExpiredMetric
TestNeighborhoodHandlerListsPublicSelectors
TestNeighborhoodHandlerReturnsPublicMarketWithProvenance
TestNeighborhoodHandlerReturnsOrderedEightWeekHistory
TestNeighborhoodHandlerRejectsInvalidWeeks
```

`TestGetMarketReassessesFreshnessAtReadTime` fixes `now` at 31 days after `InventoryCollectedAt` and asserts `expired` even if the stored metric was calculated as `current`.

- [ ] **Step 2: Run the focused tests and verify existing endpoints cannot satisfy them**

Run:

```bash
go test ./internal/application/neighborhood ./internal/interfaces/http/handler ./internal/interfaces/http/router -run 'Test(ListNeighborhoods|ListWatchlistReassesses|LatestMetricReassesses|GetMarket|GetMetricHistory|NeighborhoodHandler)' -v
```

Expected: FAIL because the current service exposes only `LatestMetric` and the legacy metrics route.

- [ ] **Step 3: Define page-oriented public types**

Add these exact service methods:

```go
type ListNeighborhoodsQuery struct{}
func (s *Service) ListNeighborhoods(context.Context, ListNeighborhoodsQuery) ([]Neighborhood, error)

func NewService(repo Repository) *Service
func NewServiceWithClock(repo Repository, now func() time.Time) *Service

type GetMarketQuery struct { NeighborhoodID string }
type DataSourceSummary struct {
	ID string `json:"id"`
	Name string `json:"name"`
	SourceType string `json:"sourceType"`
	City string `json:"city"`
}
type MarketMetric struct {
	ID string `json:"id"`
	CollectionRunID string `json:"collectionRunId"`
	InventoryCollectionRunID *string `json:"inventoryCollectionRunId"`
	ListedHomes int `json:"listedHomes"`
	PriceCutHomes int `json:"priceCutHomes"`
	AvgDaysOnMarket *float64 `json:"avgDaysOnMarket"`
	ListingPriceMin *float64 `json:"listingPriceMin"`
	ListingPriceMax *float64 `json:"listingPriceMax"`
	TransactionPriceMin *float64 `json:"transactionPriceMin"`
	TransactionPriceMax *float64 `json:"transactionPriceMax"`
	TransactionMomentum domainneighborhood.TransactionMomentum `json:"transactionMomentum"`
	TargetLayoutSupply int `json:"targetLayoutSupply"`
	ListedHomesChangePct *float64 `json:"listedHomesChangePct"`
	CalculatedAt time.Time `json:"calculatedAt"`
}
type MarketReadModel struct {
	Neighborhood Neighborhood `json:"neighborhood"`
	Metric *MarketMetric `json:"metric"`
	Signal domainneighborhood.SignalResult `json:"signal"`
	Sources []DataSourceSummary `json:"sources"` // sources contributing to Metric only
	LatestRunSource *DataSourceSummary `json:"latestRunSource,omitempty"`
	LatestObservedAt *time.Time `json:"latestObservedAt"`
	InventoryCollectedAt *time.Time `json:"inventoryCollectedAt"`
	ListingSampleCount int `json:"listingSampleCount"`
	TransactionSampleCount int `json:"transactionSampleCount"`
	Coverage domainneighborhood.Coverage `json:"coverage"`
	Freshness domainneighborhood.Freshness `json:"freshness"`
	QualityState domainneighborhood.MarketQualityState `json:"qualityState"`
	QualityWarnings []domainneighborhood.QualityWarning `json:"qualityWarnings"`
}
func (s *Service) GetMarket(context.Context, GetMarketQuery) (MarketReadModel, error)

type ListMetricHistoryQuery struct { NeighborhoodID string; Weeks int }
func (s *Service) ListMetricHistory(context.Context, ListMetricHistoryQuery) ([]MetricSnapshot, error)
```

`MarketMetric` is a JSON projection, not literal embedding in implementation: expose numerical metric fields, trigger/inventory run IDs, and calculation time, but omit stored `Coverage`, `Freshness`, `QualityState`, `QualityWarnings`, and duplicated sample counts. Current effective quality and sample counts appear once at the `MarketReadModel` top level. Public `DataSourceSummary` intentionally excludes source notes and raw import content. `Sources` names only sources contributing to the metric; `LatestRunSource` identifies a newer pending/partial run without implying that its data contributed to an older metric. Unknown neighborhood IDs return `ErrNeighborhoodNotFound`; a known neighborhood with no run returns the explicit insufficient-data model, and a known neighborhood with no history returns an empty slice.

Extend `handler.NeighborhoodApplication` and `app.NeighborhoodApplication` with the three new methods above. Keep `LatestMetric` on both interfaces even after deleting the public `/metrics` handler because `router.New` passes the same statically typed application into the default protected decision service, whose `NeighborhoodReader` requires it. Update all affected handler, router, and platform test stubs in this task so they satisfy the expanded interface.

When there is no metric, return a successful read model with `Metric=nil`, `Signal.Status="数据不足"`, `QualityState="insufficient_data"`, and no price fallback. If the latest run is partial, expose its `Coverage`, `LatestObservedAt`, and `metric_refresh_pending` warning together with `no_full_inventory`; if there is no collection run at all, use `Coverage="unknown"` and `Freshness="unknown"`. Do not convert absent market evidence into a 404 or zero-valued prices.

- [ ] **Step 4: Implement repository reads and JSON mapping**

Declare `neighborhood.ErrCollectionRunNotFound = errors.New("collection_run_not_found")` and the exact state type:

```go
type LatestCollectionRunState struct {
	ID string
	DataSourceID string
	Coverage domainneighborhood.Coverage
	CollectedAt time.Time
	MetricStatus appcollection.MetricStatus
}
```

Add these exact methods to the application repository boundary:

```go
ListNeighborhoods(context.Context) ([]Neighborhood, error)
ListMetricHistory(context.Context, neighborhoodID string, since time.Time) ([]MetricSnapshot, error)
LatestCollectionRunState(context.Context, neighborhoodID string) (LatestCollectionRunState, error)
```

The last method returns the type above for the latest completed run, or the new sentinel. `NeighborhoodRepository` lists selectors by `area, name, id`, delegates metric/history reads to the sqlmetric reader, loads distinct source summaries for a metric's provenance IDs, and returns that latest collection-run state. When no metric exists, load the latest run's source summary so a partial-only response still has provenance. If that run is newer than the latest metric or is `pending`/`failed`, the service sets `HasNewerUncalculatedRun`, appends `metric_refresh_pending`, and forces low confidence without adding the pending source to an older metric's contributing `Sources`.

For every current-market read, call `AssessQuality` once with the injected clock and latest-run state, create the signal with that assessment, and populate top-level coverage/freshness/quality fields from the same value. Map `MarketMetric` from numerical/provenance fields only, so it cannot contradict current effective quality with stored-at-calculation labels. `ListMetricHistory` intentionally returns stored snapshots including their historical quality without recomputing it. Implement `LatestMetric` by using the same current-market helper and returning a private effective snapshot/signal for internal consumers; update `ListWatchlist` to use that helper per item so protected watchlist and action-window consumers cannot retain an old sufficient signal. The in-memory repository must offer the same empty-but-explicit market behavior.

Use `NewServiceWithClock` in freshness/history tests and `NewService` in runtime composition. Update `metricResponse` to use pointer values for nullable prices/days. Add a new `marketResponse` that exposes every `MarketReadModel` field. Map `weeks` through `strconv.Atoi`; accept 1-52 only, default to 8 when omitted, and compute the history lower bound as `now.AddDate(0, 0, -7*weeks)`.

- [ ] **Step 5: Register public routes and remove the legacy route**

In the public API group register:

```go
api.GET("/neighborhoods", neighborhoodHandler.ListNeighborhoods)
api.GET("/neighborhoods/:id/market", neighborhoodHandler.GetMarket)
api.GET("/neighborhoods/:id/metrics/history", neighborhoodHandler.ListMetricHistory)
```

Remove `/neighborhoods/:id/metrics` from router tests and implementation. Neither new route uses `AccessAuth`.

- [ ] **Step 6: Verify public market behavior**

Run:

```bash
go test ./internal/application/neighborhood ./internal/interfaces/http/handler ./internal/interfaces/http/router -v
go test -race ./internal/application/neighborhood ./internal/interfaces/http/handler ./internal/interfaces/http/router
go test ./internal/platform/app ./...
git diff --check
```

Expected: PASS. Public market APIs must return no demo fallback values and must represent no metric as insufficient data.

- [ ] **Step 7: Commit public market reads**

```bash
git add internal/application/neighborhood internal/infrastructure/postgres/gorm internal/interfaces/http/handler internal/interfaces/http/router internal/platform/app
git commit -m "feat: expose traceable market reads"
```

---

### Task 7: Cut Over To Trusted Sources And JSON Import APIs

**Files:**
- Modify: `internal/application/collection/ports.go`
- Modify: `internal/application/collection/imports.go`
- Modify: `internal/application/collection/imports_test.go`
- Create: `internal/interfaces/http/handler/errors.go`
- Create: `internal/interfaces/http/handler/admin_data_sources.go`
- Create: `internal/interfaces/http/handler/admin_data_sources_test.go`
- Modify: `internal/interfaces/http/handler/admin_imports.go`
- Modify: `internal/interfaces/http/handler/admin_imports_test.go`
- Modify: `internal/interfaces/http/router/router.go`
- Modify: `internal/interfaces/http/router/router_test.go`
- Modify: `internal/platform/app/app.go`
- Modify: `internal/platform/app/app_test.go`
- Modify: `internal/platform/app/runtime.go`
- Modify: `internal/platform/app/runtime_test.go`

**Interfaces:**
- Produces protected `POST/GET /admin/api/data-sources`, `POST /admin/api/imports/json`, and `GET /admin/api/imports/{id}`.
- Consumes the normalized collection application contract from Task 3 and source/selector models from Task 6.
- Preserves the structured error envelope for all HTTP errors.

- [ ] **Step 0: Replace the legacy collection application in the same cutover**

After Task 4 provides `TrustedRepository`, update `Service` to depend on it and add:

```go
func NewService(repo TrustedRepository, now func() time.Time, newID func() string) *Service

func (s *Service) ImportCollectionRun(ctx context.Context, command ImportCollectionRunCommand) (ImportCollectionRunResult, error) {
	normalized, issues := validateAndNormalize(command, s.now().UTC())
	if len(issues) > 0 { return ImportCollectionRunResult{}, &ValidationError{Issues: issues} }

	sourceExists, err := s.repo.DataSourceExists(ctx, normalized.DataSourceID)
	if err != nil { return ImportCollectionRunResult{}, fmt.Errorf("%w: %v", ErrImportFailed, err) }
	if !sourceExists { return ImportCollectionRunResult{}, ErrDataSourceNotFound }

	neighborhoodExists, err := s.repo.NeighborhoodExists(ctx, normalized.NeighborhoodID)
	if err != nil { return ImportCollectionRunResult{}, fmt.Errorf("%w: %v", ErrImportFailed, err) }
	if !neighborhoodExists { return ImportCollectionRunResult{}, ErrNeighborhoodNotFound }

	saved, err := s.repo.SaveCollectionRun(ctx, normalized.NewBatch(s.newID(), s.newID))
	if err != nil { return ImportCollectionRunResult{}, fmt.Errorf("%w: %v", ErrImportFailed, err) }
	return ImportCollectionRunResult{
		Run: saved.Run,
		ListingCount: len(normalized.Listings),
		TransactionCount: len(normalized.Transactions),
		IdempotentReplay: !saved.Created,
		MetricRefreshStatus: MetricStatusPending,
	}, nil
}

func (s *Service) CreateDataSource(ctx context.Context, command CreateDataSourceCommand) (DataSource, error)
func (s *Service) ListDataSources(ctx context.Context) ([]DataSource, error)
func (s *Service) GetCollectionRun(ctx context.Context, query GetCollectionRunQuery) (CollectionRunDetail, error)
```

`NewService` defaults nil clocks/ID generators to `time.Now`/`uuid.NewString`, carries no metric refresher, and is the constructor used by the router fallback. New imports return `MetricStatusPending` without calculating metrics until Task 9. Task 7 acceptance tests must assert that the accepted run is durable and that public market reads expose `metric_refresh_pending`; they must not expect a usable metric yet.

In this same task delete `ImportManualListings`, its command/result, `ManualListingRecord`, and the optional metric-calculator parameter from the production collection service. Keep the now-unused legacy `Repository`, `RawCollectionRecord`, `ListingSnapshot`, GORM `SaveImport`, and old sqlmetric methods temporarily so this commit and Tasks 8-10 remain buildable; no router or runtime interface may reference them. Task 11 deletes that compatibility surface with the legacy tables. Update all handler/router/runtime interfaces and test stubs in this task before running the compiler.

- [ ] **Step 1: Write failing source, application, and JSON import tests**

Add these tests before changing routes:

```text
TestDataSourcesCreateAndList
TestDataSourcesRejectsInvalidFields
TestAdminImportsJSONNormalizesPayload
TestAdminImportsJSONReturnsCreatedRun
TestAdminImportsJSONReturnsOKForReplay
TestAdminImportsReturnsValidationDetails
TestAdminImportsReturnsNotFoundForMissingSelection
TestGetImportDetailReturnsRawTraceability
TestProtectedDataSourceAndImportRoutesRequireAccessToken
TestRouterFallbackImportPublishesPendingMarketProvenance
TestImportCollectionRunNormalizesListingAndTransaction
TestImportCollectionRunRejectsInvalidRecordWithoutRepositoryWrite
TestImportCollectionRunReturnsExistingRunForReplay
TestCreateDataSourceTrimsAndValidatesFields
TestGetCollectionRunReturnsRepositoryDetail
```

For a semantic row failure, assert exactly:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "one or more import fields are invalid",
    "details": [
      {
        "row": 1,
        "field": "listingPrice",
        "code": "required",
        "message": "listingPrice is required for listing records"
      }
    ]
  }
}
```

The handler must pass raw request bytes unchanged to the application and set format to `json`. A replay test asserts 200 and `idempotentReplay=true`, while a new import asserts 201 and `idempotentReplay=false`.

- [ ] **Step 2: Run handler/router tests and verify the old route fails**

Run:

```bash
go test ./internal/application/collection ./internal/interfaces/http/handler ./internal/interfaces/http/router -run 'Test(DataSources|AdminImports|GetImportDetail|Protected|RouterFallback|ImportCollectionRun|CreateDataSource|GetCollectionRun)' -v
```

Expected: FAIL because the legacy service only exposes `ImportManualListings`, source routes do not exist, and `ErrorResponse` cannot carry detail rows.

- [ ] **Step 3: Centralize structured API errors**

Move the existing error response into `errors.go` and extend it without changing its normal JSON shape:

```go
type errorResponse struct {
	Error struct {
		Code string `json:"code"`
		Message string `json:"message"`
		Details []appcollection.ValidationIssue `json:"details,omitempty"`
	} `json:"error"`
}

func writeValidationError(c *gin.Context, issues []appcollection.ValidationIssue) {
	var response errorResponse
	response.Error.Code = "validation_failed"
	response.Error.Message = "one or more import fields are invalid"
	response.Error.Details = issues
	c.JSON(http.StatusUnprocessableEntity, response)
}
```

`writeError` stays responsible for generic errors and must not serialize nil `details` as `null`.

- [ ] **Step 4: Implement protected source adapters**

Use this request contract:

```go
type createDataSourceRequest struct {
	Name string `json:"name"`
	SourceType string `json:"sourceType"`
	City string `json:"city"`
	Notes string `json:"notes"`
}
```

`POST` trims `name`, `sourceType`, `city`, and `notes`; requires name and city length 1-128, source type matching the lowercase slug regex `^[a-z][a-z0-9_]{0,63}$`, and notes length at most 2048. It maps invalid fields to 422 validation details, returns 201 with the created or idempotently existing source, and does not perform implicit imports. `GET` returns `{"items": []}` even when empty. `sourceType` records provenance metadata only and does not enable a collector.

- [ ] **Step 5: Implement JSON import and detail adapters**

Use:

```go
type jsonImportRequest struct {
	DataSourceID string `json:"dataSourceId"`
	NeighborhoodID string `json:"neighborhoodId"`
	SourceRef string `json:"sourceRef"`
	CollectedAt string `json:"collectedAt"`
	Coverage string `json:"coverage"`
	Records []jsonImportRecord `json:"records"`
}

type jsonImportRecord struct {
	RecordType string `json:"recordType"`
	SourceRecordID string `json:"sourceRecordId"`
	Layout string `json:"layout"`
	AreaSQM *float64 `json:"areaSqm"`
	ListingPrice *float64 `json:"listingPrice"`
	TransactionPrice *float64 `json:"transactionPrice"`
	TransactionDate *string `json:"transactionDate"`
	DaysOnMarket *int `json:"daysOnMarket"`
	Status *string `json:"status"`
	OriginalListingRef *string `json:"originalListingRef"`
	Attributes map[string]string `json:"attributes"`
}
```

Read and decode without losing the exact payload bytes:

```go
const maxImportBytes = 2 << 20
raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxImportBytes+1))
if err != nil {
	writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
	return
}
if len(raw) > maxImportBytes {
	writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds 2 MiB")
	return
}

decoder := json.NewDecoder(bytes.NewReader(raw))
decoder.DisallowUnknownFields()
if err := decoder.Decode(&request); err != nil {
	writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
	return
}
if err := decoder.Decode(&struct{}{}); err != io.EOF {
	writeError(c, http.StatusBadRequest, "invalid_request", "request body must contain one JSON value")
	return
}
command.RawPayload = append([]byte(nil), raw...)
```

Set `RawContentType` to `application/json`, parse `collectedAt` as RFC3339, and populate `Row: index + 1`. Map `areaSqm` and optional number pointers without replacing omission with zero. Parse a present `transactionDate` as `2006-01-02`; an invalid record date becomes a 422 issue for that row/field, while an invalid batch `collectedAt` remains 400. The handler passes the preserved bytes unchanged to the application checksum and persistence path.

Route errors exactly:

```text
400 invalid_request       malformed JSON, invalid RFC3339 timestamp, invalid body shape
404 data_source_not_found selected source is absent
404 neighborhood_not_found selected neighborhood is absent
404 import_not_found      requested detail is absent
413 payload_too_large     body exceeds 2 MiB
422 validation_failed     syntactically valid command has validation issues
500 import_failed         persistence failure
```

`GET /admin/api/imports/:id` returns run metadata, source, normalized listings/transactions, validation summary, and `rawPayloadBase64` using `base64.StdEncoding.EncodeToString`; it never logs or echoes raw input as unescaped JSON.

- [ ] **Step 6: Wire routes and runtime interfaces**

Remove `admin.POST("/imports", ...)`. Register:

```go
admin.POST("/data-sources", dataSourcesHandler.Create)
admin.GET("/data-sources", dataSourcesHandler.List)
admin.POST("/imports/json", adminImportsHandler.CreateJSON)
admin.GET("/imports/:id", adminImportsHandler.GetDetail)
```

Extend `app.CollectionApplication`, handler interfaces, stubs, and runtime wiring for source create/list, normalized import, and detail reads. Ensure every new admin route appears in the router's unauthenticated-401 table.

- [ ] **Step 7: Verify JSON admin APIs and commit**

```bash
go test ./internal/application/collection ./internal/interfaces/http/handler ./internal/interfaces/http/router ./internal/platform/app -v
go test -race ./internal/application/collection ./internal/interfaces/http/handler ./internal/interfaces/http/router ./internal/platform/app
go test ./...
git diff --check
git add internal/application/collection internal/interfaces/http/handler internal/interfaces/http/router internal/platform/app
git commit -m "feat: expose trusted json imports"
```

Expected: PASS, including 401 enforcement, 201 vs 200 replay behavior, and field-level 422 responses.

---

### Task 8: Add Bounded CSV Import Normalization

**Files:**
- Create: `internal/interfaces/http/handler/admin_imports_csv.go`
- Create: `internal/interfaces/http/handler/admin_imports_csv_test.go`
- Modify: `internal/interfaces/http/handler/admin_imports.go`
- Modify: `internal/interfaces/http/router/router.go`
- Modify: `internal/interfaces/http/router/router_test.go`

**Interfaces:**
- Produces protected `POST /admin/api/imports/csv`.
- Consumes: `ImportCollectionRunCommand` from Task 3 and the same result/error behavior as JSON imports.
- Guarantees: CSV and JSON use the same application validation and persistence path.

- [ ] **Step 1: Write failing CSV parser tests**

Implement table-driven tests for:

```text
TestAdminImportsCSVMapsListingAndTransactionRows
TestAdminImportsCSVMapsPhysicalRowAndField
TestAdminImportsCSVRejectsUnknownHeader
TestAdminImportsCSVRejectsDuplicateHeader
TestAdminImportsCSVRejectsMissingRequiredHeader
TestAdminImportsCSVRejectsMalformedCSV
TestAdminImportsCSVRejectsPayloadOverTwoMiB
TestAdminImportsCSVRejectsMoreThanFiveHundredRows
TestAdminImportsCSVReturnsReplayStatus
```

Use an order-independent header row containing exactly these required columns:

```text
record_type,source_record_id,layout,area_sqm,listing_price,transaction_price,transaction_date,days_on_market,status
```

Allow the optional `original_listing_ref` column. Reject all other headers and duplicate headers. A missing listing price in physical row 2 must return a 422 issue with `row: 2` and `field: "listing_price"`.

- [ ] **Step 2: Run CSV tests and verify the route is missing**

Run:

```bash
go test ./internal/interfaces/http/handler -run TestAdminImportsCSV -v
```

Expected: FAIL because the parser and route do not exist.

- [ ] **Step 3: Implement bounded multipart parsing**

Use `http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)` and `c.Request.ParseMultipartForm(2 << 20)`. Require form fields `dataSourceId`, `neighborhoodId`, `sourceRef`, `collectedAt`, and `coverage`, plus exactly one `file` part; reject zero or multiple `file` values with 400. After parsing, `defer c.Request.MultipartForm.RemoveAll()` when the form is non-nil. Set `RawContentType` from the uploaded part's content type, defaulting to `text/csv` only when it is blank.

Parse with `encoding/csv.Reader`:

```go
reader := csv.NewReader(file)
reader.FieldsPerRecord = -1
reader.TrimLeadingSpace = true
```

Read the entire bounded file into bytes once, then parse a new `bytes.Reader` so the exact raw bytes are what the application checksums and persists. Reject malformed CSV (including unequal field counts) with 400 `invalid_request`; do not silently pad missing cells.

- [ ] **Step 4: Normalize CSV cells without duplicating business validation**

Map cells to `ObservationInput`, setting source rows to `recordIndex + 2`. Parse decimal and ISO date syntax at the adapter boundary. A syntactically invalid decimal or date is a 422 `ValidationIssue` for its physical row and snake_case field; collect all such conversion issues before calling the application. Leave required/allowed combinations, ranges, duplicates, and dates relative to collection time to `validateAndNormalize` from Task 3. CSV does not populate `Attributes`.

Map lowercase `record_type` only to `listing` or `transaction`; translate field names into snake_case validation field names for CSV. Do not add a `price_cut` column or route-specific calculation.

- [ ] **Step 5: Register and protect the CSV route**

Register:

```go
admin.POST("/imports/csv", adminImportsHandler.CreateCSV)
```

Add it to router security tests. The handler must return the same `201`, `200`, `400`, `401`, `404`, `413`, `422`, and `500` outcomes as JSON. Task 10 publishes both adapters in one complete OpenAPI topology.

- [ ] **Step 6: Verify parity and commit**

```bash
go test ./internal/interfaces/http/handler ./internal/interfaces/http/router -v
go test -race ./internal/interfaces/http/handler ./internal/interfaces/http/router
git diff --check
git add internal/interfaces/http/handler internal/interfaces/http/router
git commit -m "feat: import trusted csv market data"
```

Expected: JSON and CSV have the same replay, validation, and persistence behavior; only their syntax adapters differ.

---

### Task 9: Refresh Metrics After Commit And Protect Decision Consumers

**Files:**
- Modify: `internal/application/collection/ports.go`
- Modify: `internal/application/collection/imports.go`
- Modify: `internal/application/collection/imports_test.go`
- Modify: `internal/application/metric/calculate_neighborhood.go`
- Modify: `internal/application/queue/tasks.go`
- Modify: `internal/infrastructure/queue/client.go`
- Modify: `internal/infrastructure/queue/handlers.go`
- Modify: `internal/infrastructure/queue/handlers_test.go`
- Modify: `internal/application/decision/query.go`
- Modify: `internal/application/decision/query_test.go`
- Modify: `internal/platform/app/app.go`
- Modify: `internal/platform/app/runtime.go`
- Modify: `internal/platform/app/app_test.go`
- Modify: `internal/platform/app/runtime_test.go`

**Interfaces:**
- Consumes: exact trigger-run calculation command from Task 5.
- Produces: an import response that reports metric completion or retryable failure truthfully, queue tasks carrying a collection run, and action windows that cannot bypass market quality.
- Guarantees: no collection rows are rolled back or duplicated when metric calculation fails after a successful transaction.

- [ ] **Step 1: Write failing post-commit and decision-gate tests**

Add:

```text
TestImportCollectionRunRecalculatesPersistedCollectionRun
TestImportCollectionRunMarksMetricFailedWithoutRollingBackRun
TestDuplicateImportRepairsMetricForExistingRun
TestImportCollectionRunWithoutRefreshKeepsMetricPending
TestMetricTaskCarriesCollectionRunID
TestSchedulerRepairUsesEmptyCollectionRunID
TestSchedulerRepairMarksResolvedRunMetricCompleted
TestGetActionWindowWaitsWhenMarketQualityIsLow
TestGetActionWindowWaitsWhenMarketQualityIsInsufficient
```

The metric-failure test must prove `SaveCollectionRun` returned a persisted run before refresher failure, then assert the application returns a successful result with `MetricRefreshStatus="failed"`, not a 500. The duplicate repair test starts with a run whose `metric_status` is `failed`, replays the same input, and asserts exactly one run plus a successful retry.

- [ ] **Step 2: Run focused tests and verify the collection service lacks the new command**

Run:

```bash
go test ./internal/application/collection ./internal/application/decision ./internal/application/queue ./internal/infrastructure/queue ./internal/platform/app -run 'Test(ImportCollectionRun|MetricTask|SchedulerRepair|GetActionWindowWaits)' -v
```

Expected: FAIL because import does not carry an exact run to metric calculation, queue payloads do not carry it, and action-window ignores quality state.

- [ ] **Step 3: Add truthful post-commit refresh**

Use the `UpdateMetricStatus(context.Context, collectionRunID string, status MetricStatus) error` repository method and `MetricStatusPending`, `MetricStatusCompleted`, and `MetricStatusFailed` constants added in Task 4/Task 3. Add the exact collection ports:

```go
type MetricCalculator interface {
	CalculateCollectionRun(context.Context, metric.CalculateCollectionRunCommand) error
}

type MetricRepairEnqueuer interface {
	EnqueueMetricCalculateNeighborhood(ctx context.Context, neighborhoodID string, collectionRunID string, sourceID string) error
}

func NewServiceWithMetricRefresh(
	repo TrustedRepository,
	now func() time.Time,
	newID func() string,
	calculator MetricCalculator,
	repair MetricRepairEnqueuer,
) *Service
```

The production runtime uses `NewServiceWithMetricRefresh`; router fallback continues to use Task 7's `NewService`. After `saved, err := s.repo.SaveCollectionRun(...)` succeeds, build the response once and guard the optional refresh before calling it:

```go
response := ImportCollectionRunResult{
	Run: saved.Run,
	ListingCount: len(normalized.Listings),
	TransactionCount: len(normalized.Transactions),
	IdempotentReplay: !saved.Created,
	MetricRefreshStatus: MetricStatusPending,
}
if s.metricCalculator == nil {
	return response, nil
}

err := s.metricCalculator.CalculateCollectionRun(ctx, metric.CalculateCollectionRunCommand{
	NeighborhoodID: normalized.NeighborhoodID,
	CollectionRunID: saved.Run.ID,
})
if err != nil {
	refreshStatus := MetricStatusFailed
	if updateErr := s.repo.UpdateMetricStatus(ctx, saved.Run.ID, MetricStatusFailed); updateErr != nil {
		refreshStatus = MetricStatusPending
	}
	if s.metricRepair != nil {
		_ = s.metricRepair.EnqueueMetricCalculateNeighborhood(ctx, normalized.NeighborhoodID, saved.Run.ID, "import.retry")
	}
	response.MetricRefreshStatus = refreshStatus
	response.Run.MetricStatus = refreshStatus
	return response, nil
}
response.MetricRefreshStatus = MetricStatusCompleted
response.Run.MetricStatus = MetricStatusCompleted
return response, nil
```

`CalculateCollectionRun` marks `metric_status=completed` through the metric repository after its idempotent upsert, including for an empty-ID scheduler repair. An import result must contain its durable collection run even when refresh or its status update is pending. The handler includes `metricRefreshStatus` in both 201 and 200 responses. A calculated failure returns `failed`; only an unsuccessful failure-status write returns `pending` because the durable status is then unknown. No post-commit failure returns a false 500. Do not put calculation inside the GORM import transaction or report a persisted batch as rejected.

- [ ] **Step 4: Carry exact run IDs through asynchronous repair**

Change the metric task payload to:

```go
type MetricCalculateNeighborhoodPayload struct {
	NeighborhoodID string `json:"neighborhoodId"`
	CollectionRunID string `json:"collectionRunId,omitempty"`
	SourceID string `json:"sourceId"`
}
```

Import-triggered tasks use the run ID. Scheduler repair tasks deliberately use an empty run ID, which resolves latest completed data and upserts rather than adds a duplicate history row. Change every queue and runtime boundary to the same signature:

```go
EnqueueMetricCalculateNeighborhood(ctx context.Context, neighborhoodID string, collectionRunID string, sourceID string) error
```

Change the queue handler's `MetricCalculator`, `app.MetricApplication`, and all related stubs to `CalculateCollectionRun(context.Context, metric.CalculateCollectionRunCommand) error`. The handler passes both payload IDs into that command. Keep the legacy `CalculateNeighborhood(ctx, neighborhoodID)` implementation unused but buildable until Task 11 removes it.

- [ ] **Step 5: Propagate quality into action-window recommendations**

Before calling `RecommendActionWindow`, inspect `metric.Signal.QualityState`. For `low_confidence` or `insufficient_data`, return:

```go
domaindecision.ActionWindowResult{
	Action: domaindecision.ActionWait,
	Confidence: domaindecision.ConfidenceLow,
	Summary: "目标小区的市场数据尚不足以支持买入或议价建议，先补充完整且新鲜的数据。",
	Checklist: []string{"补充最新完整挂牌和近 90 天成交记录。", "核对数据来源、覆盖范围和采集时间。"},
	Risks: []string{"不完整或过期样本会放大单套房源对判断的影响。"},
}
```

This is a normal protected 200 recommendation, not an API error. Do not let a safe budget bypass low-quality market evidence.

- [ ] **Step 6: Verify refresh and consumer gates**

```bash
go test ./internal/application/collection ./internal/application/metric ./internal/application/decision ./internal/application/queue ./internal/infrastructure/queue ./internal/platform/app -v
go test -race ./internal/application/collection ./internal/application/metric ./internal/application/decision ./internal/infrastructure/queue
git diff --check
git add internal/application internal/infrastructure/queue internal/platform/app
git commit -m "feat: refresh market metrics after import"
```

Expected: a failed metric refresh is observable and repairable without duplicating or hiding the accepted import; action-window returns low-confidence wait for poor market data.

---

### Task 10: Publish The Stage 2 OpenAPI Contract And Generated Types

**Files:**
- Modify: `api/openapi.yaml`
- Modify: `api/openapi_contract_test.go`
- Regenerate: `apps/web/src/lib/generated-api.d.ts`

**Interfaces:**
- Produces: one structural contract for all Stage 2 public and protected routes.
- Consumes: runtime route topology from Tasks 6-9.
- Preserves: `AccessBearerAuth`, shared `AccessRequired`, and structured `ErrorResponse` behavior from Stage 1.

- [ ] **Step 1: Write failing OpenAPI structure tests**

Replace the Stage 1 exact-operation table with one that asserts these operations and exact protection:

```text
Public:
GET  /api/v1/neighborhoods
GET  /api/v1/neighborhoods/{id}
GET  /api/v1/neighborhoods/{id}/market
GET  /api/v1/neighborhoods/{id}/metrics/history

Protected:
POST /api/v1/capacity/calculations
GET  /api/v1/capacity/calculations/{id}
POST /api/v1/neighborhoods
POST /api/v1/watchlist/items
GET  /api/v1/watchlist
GET  /api/v1/decision/action-window
POST /admin/api/data-sources
GET  /admin/api/data-sources
POST /admin/api/imports/json
POST /admin/api/imports/csv
GET  /admin/api/imports/{id}
```

Assert that the obsolete `/admin/api/imports` and `/api/v1/neighborhoods/{id}/metrics` paths are absent. Add structural tests for JSON/CSV request media types, 200 replay, 201 new import, `metricRefreshStatus` enum `[pending, completed, failed]`, 413, 422 `ValidationErrorResponse`, data-source selection, public market provenance, nullable price ranges, quality state/warnings, `weeks` min/max/default, and raw payload base64 detail.

- [ ] **Step 2: Run the contract test and verify Stage 1 topology fails**

Run:

```bash
go test ./api -run TestOpenAPIContract -v
```

Expected: FAIL because the current document still has old import and metrics paths and lacks Stage 2 schemas.

- [ ] **Step 3: Update OpenAPI schemas and security**

Use the shared `AccessRequired` response for every protected operation. Add:

```yaml
ValidationIssue:
  type: object
  required: [field, code, message]
  properties:
    row:
      type: integer
      minimum: 1
    field:
      type: string
    code:
      type: string
    message:
      type: string
ValidationErrorResponse:
  type: object
  required: [error]
  properties:
    error:
      type: object
      required: [code, message, details]
      properties:
        code: { type: string, enum: [validation_failed] }
        message: { type: string, enum: [one or more import fields are invalid] }
        details:
          type: array
          items: { $ref: '#/components/schemas/ValidationIssue' }
```

Add `DataSource`, `CreateDataSourceRequest`, `CollectionRunSummary`, `CollectionRunDetail`, `ListingObservation`, `TransactionObservation`, `ImportJSONRequest`, `ImportCSVRequest`, `MarketMetric`, `MarketResponse`, `MetricHistoryResponse`, `MarketQuality`, and `MarketProvenance`. `MarketMetric` excludes coverage, freshness, quality state, and warnings; `MarketResponse` carries those current reassessed values once at top level. Money schema descriptions must state `万元`; ranges are nullable where no eligible data exists. Transaction momentum permits `unknown`, `weak`, `stable`, or `strong`, and insufficient signals permit `unknown` supply pressure and target-layout scarcity.

Each successful import response has `collectionRun`, `listingObservationCount`, `transactionObservationCount`, `idempotentReplay`, and `metricRefreshStatus`. Detail uses `rawPayloadBase64` rather than raw object payload.

- [ ] **Step 4: Regenerate TypeScript types and verify consistency**

Run:

```bash
pnpm --dir apps/web generate:api
pnpm --dir apps/web typecheck
go test ./api -v
git diff --check
```

Expected: generated types compile and the Go structural contract agrees with the public/protected route topology.

- [ ] **Step 5: Commit the Stage 2 API contract**

```bash
git add api/openapi.yaml api/openapi_contract_test.go apps/web/src/lib/generated-api.d.ts
git commit -m "docs: describe trusted market data api"
```

---

### Task 11: Run The End-To-End Stage Gate And Document Manual Imports

**Files:**
- Create: `internal/platform/app/e2e_market_test.go`
- Modify: `internal/platform/app/e2e_test.go`
- Modify: `scripts/verify-stack.sh`
- Modify: `README.md`
- Modify: `docker-compose.yml`
- Modify: `internal/infrastructure/config/config.go`
- Modify: `internal/infrastructure/config/config_test.go`
- Modify: `internal/platform/app/runtime.go`
- Modify: `internal/platform/app/runtime_test.go`
- Modify: `internal/platform/app/app_test.go`
- Modify: `internal/infrastructure/postgres/gorm/neighborhood_repository.go`
- Modify: `internal/infrastructure/postgres/gorm/models.go`
- Modify: `internal/infrastructure/postgres/gorm/collection_repository.go`
- Modify: `internal/infrastructure/postgres/gorm/collection_repository_test.go`
- Modify: `internal/application/collection/ports.go`
- Modify: `internal/application/metric/ports.go`
- Modify: `internal/application/metric/calculate_neighborhood.go`
- Modify: `internal/application/metric/calculate_neighborhood_test.go`
- Modify: `queries/neighborhood_metrics.sql`
- Regenerate: `internal/infrastructure/postgres/sqlc/models.go`
- Regenerate: `internal/infrastructure/postgres/sqlc/neighborhood_metrics.sql.go`
- Modify: `internal/infrastructure/postgres/sqlmetric/repository.go`
- Modify: `internal/infrastructure/postgres/sqlmetric/repository_test.go`
- Modify: `migrations/000001_initial_schema.up.sql`
- Modify: `migrations/000001_initial_schema.down.sql`
- Modify: `migrations/embed_test.go`

**Interfaces:**
- Produces: a Compose-backed Stage 2 proof that imports become traceable public market data.
- Consumes: all Stage 2 routes, source/run persistence, metric refresh, and quality gate.
- Preserves: Stage 1 readiness/auth smoke tests and Go-only build path.

- [ ] **Step 1: Write the environment-gated end-to-end test**

Create `TestE2EImportThenQueryMarketAndHistory`. Skip only when `PROPULSE_E2E_BASE_URL` or `PROPULSE_E2E_ACCESS_TOKEN` is absent. The test must:

1. authenticate and create a data source plus neighborhood;
2. import an initial full run with at least five active listings and three transactions;
3. query public market without a token and assert that the first full run already yields full/current, sufficient quality and a non-empty metric;
4. import a newer partial run and assert public market inventory remains from the full run, `latestObservedAt` is newer, and quality is low confidence with `partial_coverage` warning;
5. import a later full run with the same listing IDs and one lower price, then assert derived `priceCutHomes > 0`;
6. include one 91-day-old transaction in that final full run and assert it is excluded from transaction sample count/range;
7. replay the final full import with identical source reference and raw bytes; assert 200, the same collection-run ID, and no duplicate history item;
8. query eight-week history without a token and assert oldest-first snapshots with no duplicate collection-run IDs;
9. assert a request to `/admin/api/imports/{id}` without a token is 401.

Use generated UUIDs and a timestamp rounded to seconds so identity and order assertions are deterministic.

- [ ] **Step 2: Run the test before wiring the Stage 2 script and verify it skips or fails for missing routes**

Run:

```bash
go test ./internal/platform/app -run TestE2EImportThenQueryMarketAndHistory -v
```

Expected: SKIP without a running Compose stack; with a Stage 1 stack it must FAIL because Stage 2 routes are absent.

- [ ] **Step 3: Extend the full-stack script safely**

After existing readiness and authenticated watchlist checks, run:

```bash
PROPULSE_E2E_BASE_URL=http://127.0.0.1:18080 \
PROPULSE_E2E_ACCESS_TOKEN=local-access-token \
go test ./internal/platform/app -run 'TestE2E(Smoke|ImportThenQueryMarketAndHistory)' -v
```

Keep bounded curl timeouts. Add explicit checks that public `GET /api/v1/neighborhoods` is 200 and unauthenticated `GET /admin/api/data-sources` is exactly 401. Do not add a volume reset to this script; callers control database lifecycle.

When Docker is available, make the PostgreSQL repository claims executable rather than skipped. In `verify-stack.sh`, after Compose is healthy and before the HTTP E2E test, create a disposable database distinct from the live `propulse` database and run the integration suites against it:

```bash
docker compose exec -T postgres psql -U propulse -d postgres -v ON_ERROR_STOP=1 \
  -c 'DROP DATABASE IF EXISTS propulse_stage2_test;'
docker compose exec -T postgres psql -U propulse -d postgres -v ON_ERROR_STOP=1 \
  -c 'CREATE DATABASE propulse_stage2_test;'
PROPULSE_TEST_DATABASE_URL='postgres://propulse:propulse@127.0.0.1:15432/propulse_stage2_test?sslmode=disable' \
  go test ./internal/infrastructure/postgres/gorm ./internal/infrastructure/postgres/sqlmetric -v
```

Those tests apply the embedded migration to the fresh database and must include Task 4 atomic rollback/concurrency coverage plus Task 5 aggregation/as-of/history coverage. Do not point `PROPULSE_TEST_DATABASE_URL` at the live Compose application database.

- [ ] **Step 4: Remove automatic demo market seeds from the Compose path**

Remove `PROPULSE_SEED_DEMO_DATA` from all Compose services, remove `SeedDemoData`, remove `seedDemoDataFunc`, remove the `SeedDemoData` config field and environment parsing, and replace runtime/app tests with explicit source/run fixtures. Update `internal/platform/app/runtime.go`, `runtime_test.go`, and `app_test.go` in the same commit so no composition path can create fabricated market metrics. This prevents publicly deployed Compose from presenting fabricated market metrics as trusted data.

`README.md` must state that an empty market is expected until a user creates a source and imports a full batch, describe `full` versus `partial`, show one authenticated JSON import example, and state that current inventory requires a full run while the API exposes freshness and warnings.

- [ ] **Step 5: Contract the undeployed schema and remove legacy compatibility code**

Rewrite the same `000001_initial_schema.up.sql` into its final Stage 2 form before its first deployment:

```sql
-- Remove the two legacy collection tables and their listing-snapshot indexes.
-- neighborhood_metrics keeps these trusted fields:
collection_run_id UUID NOT NULL,
inventory_collection_run_id UUID,
source_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
listing_sample_count INT NOT NULL CHECK (listing_sample_count >= 0),
transaction_sample_count INT NOT NULL CHECK (transaction_sample_count >= 0),
listed_homes_change_pct NUMERIC(8,2),
coverage TEXT NOT NULL CHECK (coverage IN ('full', 'partial')),
freshness TEXT NOT NULL CHECK (freshness IN ('unknown', 'current', 'stale', 'expired')),
quality_state TEXT NOT NULL CHECK (quality_state IN ('sufficient', 'low_confidence', 'insufficient_data')),
latest_observed_at TIMESTAMPTZ NOT NULL,
inventory_collected_at TIMESTAMPTZ,
quality_warnings JSONB NOT NULL DEFAULT '[]'::jsonb,
FOREIGN KEY (collection_run_id, neighborhood_id)
  REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
FOREIGN KEY (inventory_collection_run_id, neighborhood_id)
  REFERENCES collection_runs(id, neighborhood_id)
  ON DELETE SET NULL (inventory_collection_run_id),
UNIQUE (collection_run_id)
```

Keep `inventory_collection_run_id`, `inventory_collected_at`, `avg_days_on_market`, and all price-range columns nullable because a persisted partial-only run is an explicit insufficient-data snapshot. Keep `price_cut_homes` as a derived metric count, but remove the legacy input column named `price_cut` with `listing_snapshots`. The final down migration drops `neighborhood_metrics`, `transaction_observations`, `listing_observations`, `collection_runs`, `data_sources`, watchlist, neighborhoods, and capacity in reverse dependency order; it contains no legacy table drops.

Replace the Task 1 migration assertion with final required strings for the four trusted tables, `collection_run_id UUID NOT NULL`, `latest_observed_at TIMESTAMPTZ NOT NULL`, `FOREIGN KEY (collection_run_id, neighborhood_id)`, `ON DELETE SET NULL (inventory_collection_run_id)`, `UNIQUE (collection_run_id)`, and the trusted observation indexes. Add this negative assertion:

```go
for _, forbidden := range []string{
	"CREATE TABLE raw_collection_records",
	"CREATE TABLE listing_snapshots",
	"price_cut BOOLEAN",
} {
	if strings.Contains(string(body), forbidden) {
		t.Fatalf("final initial schema still contains legacy %q", forbidden)
	}
}
```

Delete the now-unused `Repository`, `RawCollectionRecord`, `ListingSnapshot`, and `SaveImport` compatibility path from collection, GORM, and the router in-memory repository/tests. Remove the legacy raw/snapshot maps and `SaveImport` method from `collection_memory.go` while retaining its trusted source/run/observation state. Delete `AggregateListingSnapshots`, `InsertNeighborhoodMetric`, `CalculateNeighborhood(ctx, neighborhoodID)`, the legacy aggregate tests, and their SQL definitions. Run `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` after deleting old query definitions; the generated model file must no longer contain `RawCollectionRecord` or `ListingSnapshot`.

Run:

```bash
go test ./migrations ./internal/application/collection ./internal/application/metric ./internal/infrastructure/postgres/gorm ./internal/infrastructure/postgres/sqlmetric -v
go test ./...
git diff --check
```

Expected: the final undeployed migration contains only trusted market-data tables and every active Go package uses only the normalized collection and trusted metric contracts.

- [ ] **Step 6: Run all Stage 2 gates**

```bash
pnpm --dir apps/web install --frozen-lockfile
pnpm --dir apps/web verify
pnpm --dir apps/web build:web
git diff --exit-code -- apps/web/embed/static
test -z "$(git status --porcelain=v1 --untracked-files=all -- apps/web/embed/static)"
go test ./...
go test -race ./...
go vet ./...
go mod verify
go build -o /tmp/propulse-stage2 ./cmd/propulse
/tmp/propulse-stage2 --help
rm -f /tmp/propulse-stage2
docker compose config --quiet
git diff --check
```

When the Docker daemon is available, run:

```bash
docker compose down -v
bash scripts/verify-stack.sh
```

If Docker socket access is unavailable, record that limitation and do not claim live PostgreSQL/Redis/E2E execution passed.

- [ ] **Step 7: Review scope, delete stale names, and commit**

Run:

```bash
if rg -n 'ImportManualListings|ManualListingRecord|RawCollectionRecord|ListingSnapshot|raw_collection_records|listing_snapshots' \
  --glob '!docs/superpowers/**' --glob '!apps/web/embed/static/**' --glob '!migrations/embed_test.go' .; then exit 1; fi
if rg -n '\bprice_cut\b' migrations queries internal/infrastructure \
  --glob '!migrations/embed_test.go' --glob '!**/*_test.go'; then exit 1; fi
if rg -n 'PROPULSE_SEED_DEMO_DATA|SeedDemoData|seedDemoDataFunc' \
  --glob '!docs/superpowers/**' --glob '!apps/web/embed/static/**' .; then exit 1; fi
if rg -n 'admin\.POST\("/imports"|/admin/api/imports:|neighborhoods/:id/metrics"|/api/v1/neighborhoods/\{id\}/metrics:' \
  internal api/openapi.yaml; then exit 1; fi
git status --short
git diff --stat 7bbf114
git diff --check 7bbf114
```

Expected: no legacy collection API/schema names remain in active code; only intentional Stage 2 changes are present.

```bash
git add README.md docker-compose.yml scripts migrations queries api internal/platform/app internal/infrastructure/config internal/application/collection internal/application/metric internal/infrastructure/postgres/gorm internal/infrastructure/postgres/sqlc internal/infrastructure/postgres/sqlmetric internal/interfaces/http/router
git commit -m "refactor: complete trusted market data cutover"
```

---

## Post-Stage Review

After Task 11:

- review the Stage 2 diff for API/auth regressions and idempotency races;
- confirm one source/run replay cannot create duplicate observations or metrics;
- confirm partial, stale, expired, and insufficient inputs suppress confident market and action-window recommendations;
- confirm public market reads expose provenance without exposing raw source payloads;
- confirm the repository remains a one-Node-package, Go-only-build repository;
- do not begin Stage 3 frontend migration or Stage 4 decision-review schema work in this stage.
