# Task 2 Report: Static Next Export And Embedded Frontend Server

## What you implemented

- Enabled static Next export in `next.config.mjs` with `output: "export"`.
- Added package script `build:web` to run the Next production build and sync exported assets into `backend/web/static/`.
- Added package script `verify` to run frontend typecheck, lint, and tests.
- Added `scripts/sync-static-web.mjs` to replace `backend/web/static/` with the generated `out/` directory contents.
- Added Go package `backend/web` with embedded static assets via `//go:embed all:static`.
- Implemented `Embedded() fs.FS` so callers receive a filesystem rooted at the embedded `static` directory, allowing `fs.Stat(Embedded(), "index.html")` to succeed.
- Added placeholder `backend/web/static/.gitkeep` so the embed target exists in git before build output is synced.

## What you tested and results

- `pnpm build:web`
  - Passed.
  - Confirmed `backend/web/static/index.html` exists after sync.
- `cd backend && go test ./web -run TestEmbeddedWebContainsIndex -v`
  - Passed.
- `cd backend && go test ./...`
  - Passed.
- `pnpm verify`
  - Passed.
  - Frontend typecheck, lint, and Vitest suite all green.

## TDD Evidence: RED and GREEN commands and relevant output

### RED

Command:

```bash
cd backend
go test ./web -run TestEmbeddedWebContainsIndex -v
```

Output:

```text
# github.com/propulse/propulse/backend/web [github.com/propulse/propulse/backend/web.test]
web/web_test.go:9:14: undefined: Embedded
FAIL    github.com/propulse/propulse/backend/web [build failed]
FAIL
```

### GREEN

Command:

```bash
pnpm build:web
cd backend
go test ./web -run TestEmbeddedWebContainsIndex -v
```

Relevant output:

```text
backend/web/static/index.html exists
=== RUN   TestEmbeddedWebContainsIndex
--- PASS: TestEmbeddedWebContainsIndex (0.00s)
PASS
ok      github.com/propulse/propulse/backend/web    0.002s
```

## Files changed

- `next.config.mjs`
- `package.json`
- `scripts/sync-static-web.mjs`
- `backend/web/web.go`
- `backend/web/web_test.go`
- `backend/web/static/.gitkeep`

## Self-review findings

- `Embedded()` uses `fs.Sub(embedded, "static")` exactly as required and only panics if the embedded directory is unavailable.
- The change does not add routing, SSR, server actions, or Next API route behavior.
- The copied static tree lands under `backend/web/static/` as resolved in the plan.
- No `pnpm-lock.yaml` changes were needed because no dependencies changed.

## Concerns, if any

- None.

## Fix follow-up

- Restored `backend/web/static/.gitkeep` as a tracked file and updated `scripts/sync-static-web.mjs` to rewrite the placeholder after every `pnpm build:web` sync, so the generated static tree still contains the required file after the directory is removed and recreated.
- Verified with `pnpm build:web`, `test -f backend/web/static/.gitkeep`, `cd backend && go test ./web -run TestEmbeddedWebContainsIndex -v`, and `cd backend && go test ./...`.
