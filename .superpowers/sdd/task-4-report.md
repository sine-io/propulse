# Task 4 Report

## Scope

Implemented the capacity domain, application service, HTTP handler, router wiring, and OpenAPI surface for the housing capacity calculation API.

## What Changed

- Added `backend/internal/domain/capacity` with the `PressureLevel` enum, input/output DTOs, and the Go port of the housing capacity calculation rule.
- Added `backend/internal/application/capacity` with `CreateCalculationCommand`, `GetCalculationQuery`, `CalculationRecord`, `CalculationRepository`, and the service implementation.
- Added `backend/internal/interfaces/http/handler/capacity.go` and tests for:
  - `POST /api/v1/capacity/calculations`
  - invalid JSON handling
  - `GET /api/v1/capacity/calculations/:id`
  - not found and server error cases
- Updated `backend/internal/interfaces/http/router/router.go` to expose the new API while keeping `/api/v1` and `/admin/api` unknown-route requests on JSON 404 behavior instead of falling through to frontend HTML.
- Added `backend/api/openapi.yaml` with the new create/get capacity calculation paths and the full numeric input schema.

## Verification

I personally ran:

```bash
cd backend && go test ./internal/domain/capacity ./internal/application/capacity ./internal/interfaces/http/... -v
cd backend && go test ./...
```

Both passed.

The controller had already observed the focused backend test command passing before I finished, but no earlier RED output was available to me. I am not reconstructing or inventing a failing TDD step.

## Notes

- I did not implement PostgreSQL, migrations, neighborhood logic, or frontend changes.
- I did not find any remaining functional gap in the Task 4 scope after running the full backend test suite.
