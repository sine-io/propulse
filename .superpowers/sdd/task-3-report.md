# Task 3 Report

## What you implemented

- Added Gin HTTP router construction in `backend/internal/interfaces/http/router/router.go`.
- Added request ID middleware in `backend/internal/interfaces/http/middleware/request_id.go`:
  - uses inbound `X-Request-Id` when present
  - generates a UUID otherwise
  - writes `X-Request-Id` on the response
  - stores `request_id` in Gin context
- Added zerolog request logging middleware in `backend/internal/interfaces/http/middleware/logging.go` emitting one JSON event per request with:
  - `request_id`
  - `method`
  - `path`
  - `status`
  - `latency_ms`
- Added health endpoints:
  - `GET /healthz`
  - `GET /readyz`
- Added API/admin route groups:
  - `/api/v1`
  - `/admin/api`
  - both return JSON 404s for unknown routes and do not fall back to frontend HTML
- Added embedded frontend routing:
  - `/`
  - `/calculator`
  - `/watchlist`
  - `/action-window`
  - `/neighborhoods`
  - `/methods`
  - `/templates`
- Added embedded static asset serving for `_next` assets and `icon.svg`.
- Updated `backend/internal/platform/app/app.go` so:
  - `api` starts HTTP server only
  - `serve` starts HTTP server only for now
  - `worker`, `scheduler`, `migrate up`, `migrate down` keep the existing context-wait skeleton
  - HTTP server shuts down on context cancellation

## What you tested and results

- Focused router and app tests:
  - `go test ./internal/interfaces/http/... ./internal/platform/app/... -v`
  - Result: pass
- Build verification:
  - `go build -o bin/propulse ./cmd/propulse`
  - Result: pass
- Live binary verification:
  - attempted `PROPULSE_HTTP_ADDR=:18080 ./bin/propulse api`
  - environment had `:18080` already occupied, so repeated on `:18081`
  - `curl -i http://127.0.0.1:18081/healthz`
    - result: `200 OK`, JSON body, `X-Request-Id` present
  - `curl -i http://127.0.0.1:18081/calculator`
    - result: `200 OK`, HTML body from embedded frontend
  - `curl -i http://127.0.0.1:18081/api/v1/missing`
    - result: `404 Not Found`, JSON body `{"error":"not_found"}`
  - observed server logs:
    - JSON log lines emitted with `request_id`, `method`, `path`, `status`, `latency_ms`

## TDD Evidence: RED and GREEN commands and relevant output

### RED

Command:

```bash
cd backend
go test ./internal/interfaces/http/... ./internal/platform/app/... -v
```

Relevant output:

```text
# github.com/propulse/propulse/backend/internal/interfaces/http/router [github.com/propulse/propulse/backend/internal/interfaces/http/router.test]
internal/interfaces/http/router/router_test.go:18:12: undefined: New
internal/interfaces/http/router/router_test.go:18:16: undefined: Dependencies
...
=== RUN   TestRunStartsHTTPServerForAPIMode
    app_test.go:36: server at http://127.0.0.1:44915/healthz did not become healthy before timeout
--- FAIL: TestRunStartsHTTPServerForAPIMode (3.03s)
=== RUN   TestRunStartsHTTPServerForServeMode
    app_test.go:40: server at http://127.0.0.1:36613/healthz did not become healthy before timeout
--- FAIL: TestRunStartsHTTPServerForServeMode (3.03s)
FAIL
```

### GREEN

Command:

```bash
cd backend
go test ./internal/interfaces/http/... ./internal/platform/app/... -v
```

Relevant output:

```text
=== RUN   TestHealthAndReadyRoutes
--- PASS: TestHealthAndReadyRoutes (0.00s)
=== RUN   TestAPI404DoesNotReturnFrontend
--- PASS: TestAPI404DoesNotReturnFrontend (0.00s)
=== RUN   TestFrontendRoutesServeEmbeddedHTML
--- PASS: TestFrontendRoutesServeEmbeddedHTML (0.00s)
=== RUN   TestRequestIDMiddlewareEchoesInboundHeaderAndLogsRequest
--- PASS: TestRequestIDMiddlewareEchoesInboundHeaderAndLogsRequest (0.00s)
=== RUN   TestRequestIDMiddlewareGeneratesHeaderWhenMissing
--- PASS: TestRequestIDMiddlewareGeneratesHeaderWhenMissing (0.00s)
=== RUN   TestRunStartsHTTPServerForAPIMode
--- PASS: TestRunStartsHTTPServerForAPIMode (0.00s)
=== RUN   TestRunStartsHTTPServerForServeMode
--- PASS: TestRunStartsHTTPServerForServeMode (0.00s)
PASS
```

## Files changed

- `backend/go.mod`
- `backend/go.sum`
- `backend/internal/interfaces/http/middleware/request_id.go`
- `backend/internal/interfaces/http/middleware/logging.go`
- `backend/internal/interfaces/http/router/router.go`
- `backend/internal/interfaces/http/router/router_test.go`
- `backend/internal/platform/app/app.go`
- `backend/internal/platform/app/app_test.go`

## Self-review findings

- Router behavior matches the briefed surface area and intentionally does not implement product/admin APIs beyond JSON 404s.
- API/admin paths are isolated from frontend fallback.
- Request logging emits a single zerolog JSON entry after request completion.
- `serve` and `api` both own the HTTP server path, with shutdown bound to context cancellation.
- No additional product logic or metrics endpoint was introduced.

## Concerns, if any

- Adding Gin required dependency resolution that moved `backend/go.mod` to `go 1.25.0` because current resolved transitive dependencies require a newer Go toolchain.
- The exact live verification command from the brief could not bind `:18080` in this environment because that port was already in use; equivalent verification was completed on `:18081`.
