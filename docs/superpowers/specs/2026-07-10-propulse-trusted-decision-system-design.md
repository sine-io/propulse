# Propulse Trusted Decision System Design

## 1. Objective

Propulse will become a single-user housing decision system that can be safely
deployed to the public internet. It must help the user:

- understand current neighborhood market conditions from traceable data;
- calculate a serviceable purchase or home-swap budget;
- turn budget and market signals into explainable actions;
- record decisions, outcomes, and viewing evidence over time;
- derive, test, revise, and retire personal decision rules.

The project remains a Go-first repository. A clean checkout must support:

```bash
go build ./cmd/propulse
```

The resulting binary must contain the complete static frontend without
requiring Node.js at Go build time.

## 2. Scope And Delivery Strategy

Work is divided into four independently testable stages:

1. Repository and Go infrastructure cleanup.
2. Trusted manual market-data ingestion.
3. Frontend migration from demo behavior to real APIs.
4. Decision review and personal methodology loop.

Each stage must leave the repository buildable and usable. Automatic website
collection is explicitly deferred. Its future implementations will emit the
same normalized collection batches as manual imports.

The first release is a persistent single-user product. It does not include
registration, login, multiple accounts, social features, or AI-generated
methodology summaries.

## 3. Repository And Build Architecture

### 3.1 Target Structure

```text
propulse/
├── apps/
│   └── web/
│       ├── package.json
│       ├── pnpm-lock.yaml
│       ├── node_modules/             # local only, ignored by Git
│       ├── scripts/
│       │   └── sync-embed.mjs
│       ├── src/
│       └── embed/
│           ├── embed.go
│           ├── embed_test.go
│           └── static/               # tracked Next.js export
├── api/                              # shared OpenAPI contract
├── cmd/
├── internal/
├── migrations/
├── queries/
├── scripts/
│   └── verify-stack.sh               # repository-wide verification
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

All Node package-management files and frontend-only scripts live under
`apps/web`. The repository is not a pnpm workspace because it contains only
one Node package.

The root `api` directory stays at the repository boundary because its OpenAPI
contract is consumed by both Go and TypeScript. The root `scripts` directory
contains only repository-wide or full-stack scripts.

### 3.2 Frontend Embedding

Next.js continues to use static export. `apps/web/scripts/sync-embed.mjs`
copies `apps/web/out` to `apps/web/embed/static`.

The Go package at `github.com/sine-io/propulse/apps/web/embed` embeds the
tracked static export with `go:embed`. Application composition imports that
package directly. A clean checkout therefore contains everything required by
`go build ./cmd/propulse`.

The tracked export is a deployment input, not a second frontend source tree.
CI and local verification must prove that a fresh frontend build produces the
same files as `apps/web/embed/static`.

### 3.3 Cleanup

The following items are removed:

- root `pnpm-lock.yaml` after moving it to `apps/web`;
- root `pnpm-workspace.yaml`;
- root `node_modules`;
- the obsolete `website/propulse.html` prototype and its directory;
- the empty `assets` directory;
- the old root `web` package after moving it under `apps/web/embed`;
- the redundant `000002_listing_snapshots_collection_run` migration;
- unused Chromium runtime packages until an automatic collector exists.

Historical implementation documents that describe the former `backend/`
layout remain available only when clearly marked as historical. Current
architecture documentation must use the actual paths.

### 3.4 Database And Runtime Composition

Because migrations have not been deployed, the schema is consolidated into a
clean `000001_initial_schema` migration. There is no compatibility requirement
for the current development-only schema.

The application composition root creates and owns shared PostgreSQL resources:

- one GORM connection backed by one `database/sql` pool;
- one pgx pool for sqlc queries;
- one Redis client or health probe where required.

Repositories and services receive these shared dependencies. They do not open
their own pools.

`/healthz` remains a process liveness check. `/readyz` checks required
PostgreSQL and Redis dependencies and returns a non-2xx response when the
service cannot handle its configured workload.

## 4. Access Model

The service uses a single configured user identity and one access token:

```text
PROPULSE_ACCESS_TOKEN
```

There is no compiled-in token and no default production token.

### 4.1 Public Reads

The following data is public:

- neighborhood identity and basic information;
- current market observations and calculated metrics;
- market history;
- data source, collection time, sample counts, coverage, freshness, and
  quality warnings;
- general methodology content.

### 4.2 Protected Operations And Reads

Bearer-token authentication protects:

- capacity calculations and their history;
- watchlist reads and writes;
- action-window recommendations;
- manual data imports and import administration;
- viewing notes;
- decision reviews and outcomes;
- personal decision rules.

The frontend asks the user to unlock the personal space. It stores the token
in `sessionStorage`, never in generated static files, URLs, server logs, or
persistent browser storage. A `401` response produces an unlock state rather
than demo content.

## 5. Trusted Manual Market Data

### 5.1 Data Model

`data_sources` describes a data origin:

- name;
- source type;
- city;
- optional notes;
- created and updated timestamps.

`collection_runs` describes one import batch:

- data source;
- target neighborhood;
- source reference;
- collection timestamp;
- `full` or `partial` coverage;
- import format;
- content checksum;
- raw payload or raw-file reference;
- validation summary;
- status and timestamps.

`listing_observations` stores listing-state observations:

- collection run and neighborhood;
- stable `source_listing_id`;
- layout and area;
- listing price;
- days on market;
- listing status;
- captured timestamp;
- optional structured attributes needed for comparison.

`transaction_observations` stores transactions separately:

- collection run and neighborhood;
- stable source record ID;
- layout and area;
- transaction price and date;
- optional original listing reference;
- captured timestamp.

`neighborhood_metrics` records a calculated snapshot and its provenance:

- collection run;
- listing and transaction sample counts;
- coverage type;
- freshness state;
- quality warnings;
- calculated market values and signals;
- calculation timestamp.

The existing capacity and watchlist concepts remain, but all rows use a
single stable application user ID rather than the name `demo-user`.

### 5.2 JSON And CSV Imports

Protected endpoints accept JSON and CSV:

```text
POST /admin/api/imports/json
POST /admin/api/imports/csv
GET  /admin/api/imports/{id}
```

Both formats normalize into the same collection-run command. CSV uses
`record_type` to distinguish listing and transaction rows. Its supported
columns include:

```text
record_type
source_record_id
layout
area_sqm
listing_price
transaction_price
transaction_date
days_on_market
status
```

Every import is transactional. Any invalid row rejects the whole batch, and
the response identifies the row and field. Each batch contains at most 500
records in the first release.

The tuple of source, source reference, and content checksum makes repeated
imports idempotent. Raw inputs and validation results remain traceable after
normalization.

### 5.3 Market Calculation Rules

Current inventory is calculated only from the latest `full` coverage run. A
partial run cannot claim to represent total inventory.

Price reductions are derived from successive observations with the same
`source_listing_id`; importers do not submit a trusted `priceCut` boolean.

Transaction ranges use transaction samples from the latest 90 days. Every
market response includes:

- sources;
- last collection time;
- listing and transaction sample counts;
- coverage type;
- freshness;
- quality warnings.

Freshness is defined as:

- `current`: collected within 7 days;
- `stale`: collected 8 to 30 days ago;
- `expired`: collected more than 30 days ago.

Partial, expired, or insufficient data must not produce a confident bargain
or purchase recommendation. The domain returns an insufficient-data state or
a low-confidence wait recommendation with explicit reasons.

### 5.4 Market Query API

Public market queries include:

```text
GET /api/v1/neighborhoods/{id}/market
GET /api/v1/neighborhoods/{id}/metrics/history?weeks=8
```

The market response is a page-oriented read model. It contains the latest
market snapshot, provenance, quality assessment, and explainable signal. The
history endpoint returns time-ordered snapshots suitable for trends and week
over week comparisons.

Future automatic collectors write the same normalized collection-run input
and reuse all validation, calculation, API, and frontend behavior.

## 6. Real-Data Frontend

### 6.1 Shared Client Behavior

The OpenAPI contract remains the source for generated TypeScript types. A
single client module handles:

- bearer-token injection for protected endpoints;
- unlock and `401` behavior;
- structured API errors;
- loading, empty, stale, expired, and failed states;
- data-quality metadata.

Pages never silently replace failed requests with sample market or personal
data.

### 6.2 Capacity

Go is the sole source of capacity calculations. The frontend removes its
duplicate calculation rules and reference-price display overrides.

The protected API returns the complete calculation result, reasons, record
ID, and timestamp. A user may fill the form while locked but must unlock to
submit or retrieve personal results.

### 6.3 Neighborhood Market Page

The static `/neighborhoods` route selects a neighborhood using a query
parameter such as `?id=<uuid>`. This avoids runtime dynamic routes that are
incompatible with the static export deployment.

The page displays:

- real current metrics;
- eight-week history;
- collection time and source;
- sample counts and coverage;
- freshness and quality warnings;
- explainable market status and recommended observation focus.

Controls that do not work are removed. A watchlist button is shown only when
it is connected to a protected API. Notification delivery is not claimed in
this stage.

### 6.4 Watchlist

The watchlist is protected personal data. It distinguishes:

- loading;
- locked;
- empty;
- current data;
- stale or expired data;
- server failure.

Weekly changes are computed from metric history. Hard-coded alerts and market
deltas are removed. Until outbound notification delivery exists, the page
uses the term on-page abnormal signals rather than notifications.

### 6.5 Action Window

The protected action-window query uses the latest capacity calculation and an
explicitly selected watched neighborhood. The entire factor matrix, action,
confidence, checklist, risks, and evidence come from Go.

Missing prerequisites route the user to the capacity or neighborhood flow.
Expired, partial, or insufficient market data never falls back to a simulated
recommendation.

### 6.6 Data Management

A protected `/data` page provides:

- CSV template download;
- JSON or CSV upload;
- data-source and neighborhood selection;
- collection timestamp and coverage selection;
- validation errors with row and field details;
- successful batch summary and batch-detail navigation.

### 6.7 Static Content

The home page may retain illustrative content, but every illustrative number
must be marked as an example. The general methods page remains static content
and can accept market context in links from real-data pages.

Frontend sample data may remain only inside tests and clearly labelled static
examples. It is not used as a runtime error fallback.

## 7. Decision Review And Personal Methodology

### 7.1 Review Flow

```text
market and capacity snapshot
  -> observed facts
  -> personal judgment and confidence
  -> chosen action
  -> next action and invalidation conditions
  -> actual outcome
  -> confirmed, revised, or rejected personal rule
```

### 7.2 Data Model

`decision_reviews` stores:

- immutable references or snapshots of the relevant capacity calculation,
  neighborhood metric, and collection run;
- observed facts;
- user judgment;
- confidence;
- chosen action;
- invalidation conditions;
- next-week plan;
- review period and timestamps.

`review_outcomes` stores:

- actual action taken;
- later market changes;
- what was correct or incorrect;
- resulting lesson;
- recorded timestamp.

`viewing_notes` stores:

- viewing date;
- optional source listing reference;
- asking price;
- advantages and defects;
- maximum acceptable price;
- negotiation evidence;
- exit conditions.

`personal_rules` stores:

- title and applicability conditions;
- action rule and rationale;
- supporting reviews and counterexamples;
- `draft`, `active`, or `retired` status;
- revision timestamps.

Every personal rule must cite at least one source review. Rules are user-owned
interpretations, not automatic system claims.

### 7.3 Product Surfaces

```text
/reviews
/reviews/new
/viewings
/rules
```

The watchlist surfaces pending weekly reviews instead of an unsaved textarea.
The review form preloads the relevant market change and budget state, then
asks structured questions. The methods page supplies general frameworks and
can help the user draft a rule from a completed review.

The first release does not use AI to invent or summarize personal rules. The
system organizes facts, calculations, evidence, and prompts. If AI is added
later, every conclusion must cite stored market, decision, and outcome data.

## 8. Error Handling And Trust Rules

- Authentication failures return structured `401` responses.
- Authorization configuration failures prevent protected modes from claiming
  readiness.
- Validation errors identify the exact field and, for CSV, row number.
- Duplicate imports return the existing collection run rather than creating
  duplicate observations.
- Missing data is distinct from stale, expired, partial, and server-error
  states.
- Market and decision responses always expose provenance and uncertainty.
- Logs include request and batch IDs but exclude access tokens and raw personal
  financial inputs.
- No UI displays a successful action for an operation that has not been
  persisted.

## 9. Testing And Verification

### 9.1 Go

- domain tests cover freshness, coverage, sample sufficiency, price-change
  derivation, market signals, and action-window confidence;
- application tests cover import validation, idempotency, transactions,
  personal reviews, outcomes, and rule lifecycle;
- repository tests run against PostgreSQL for migration and query behavior;
- router and handler tests cover public and protected routes, token handling,
  readiness, and structured errors;
- embed tests prove all exported routes and assets are present;
- an end-to-end test covers migrate, import, market query, capacity,
  watchlist, action window, review, outcome, and personal rule creation.

### 9.2 Frontend

- client tests cover token injection without leakage;
- component tests cover locked, loading, empty, current, stale, expired,
  insufficient, validation-error, and server-error states;
- pages are tested against real API-shaped fixtures, not runtime fallbacks;
- the production static export must build successfully;
- the generated export and tracked embed directory must match.

### 9.3 Repository Gates

The final verification set includes:

```bash
go test ./...
go vet ./...
go build ./cmd/propulse
pnpm --dir apps/web verify
pnpm --dir apps/web build:web
git diff --exit-code -- apps/web/embed/static
```

The full-stack verification additionally starts PostgreSQL and Redis, runs
migrations, checks readiness, imports a batch, queries market data, and runs
the Go end-to-end test.

## 10. Acceptance Criteria

The design is complete when:

- the repository has exactly one Node package and one local `node_modules`;
- every frontend-specific tracked file lives under `apps/web` except the
  shared OpenAPI contract;
- a clean checkout can build a frontend-containing binary with only Go;
- migrations describe one coherent undeployed schema;
- readiness reflects PostgreSQL and Redis availability;
- market numbers are traceable to a source and collection run;
- partial or old data cannot masquerade as current complete market data;
- frontend API failures never display unlabeled demo data;
- protected personal data requires the configured access token;
- a user can import weekly data, inspect changes, calculate capacity, receive
  an explainable recommendation, record a review and outcome, and maintain a
  source-backed personal rule;
- all automated verification gates pass with a clean Git worktree.
