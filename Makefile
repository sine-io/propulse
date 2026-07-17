.PHONY: build collect-fangjian verify verify-go verify-web verify-generated

SQLC_VERSION ?= v1.30.0

build:
	go build -o dist/propulse ./cmd/propulse

collect-fangjian:
	go run ./cmd/fangjian-collector --community all --output data/fangjian

verify: verify-go verify-web verify-generated

verify-go:
	go test -race ./...
	go vet ./...

verify-web:
	pnpm --dir apps/web verify
	pnpm --dir apps/web build

verify-generated:
	@tmp_dir="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	cp -a internal/infrastructure/postgres/sqlc "$$tmp_dir/sqlc"; \
	cp apps/web/src/lib/generated-api.d.ts "$$tmp_dir/generated-api.d.ts"; \
	cp -a apps/web/embed/static "$$tmp_dir/static"; \
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate; \
	pnpm --dir apps/web generate:api; \
	diff -qr "$$tmp_dir/sqlc" internal/infrastructure/postgres/sqlc; \
	diff -q "$$tmp_dir/generated-api.d.ts" apps/web/src/lib/generated-api.d.ts; \
	diff -qr --exclude=.gitkeep "$$tmp_dir/static" apps/web/embed/static; \
	diff -qr --exclude=.gitkeep apps/web/out apps/web/embed/static
