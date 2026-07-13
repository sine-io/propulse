.PHONY: build verify verify-go verify-web verify-generated

SQLC_VERSION ?= v1.31.1

build:
	go build -o dist/propulse ./cmd/propulse

verify: verify-go verify-web verify-generated

verify-go:
	go test -race ./...
	go vet ./...

verify-web:
	pnpm --dir apps/web verify
	pnpm --dir apps/web build

verify-generated:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate
	pnpm --dir apps/web generate:api
	diff -qr --exclude=.gitkeep apps/web/out apps/web/embed/static
	git diff --exit-code -- internal/infrastructure/postgres/sqlc apps/web/src/lib/generated-api.d.ts apps/web/embed/static
