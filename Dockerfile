FROM node:22-alpine AS node-deps

ENV NEXT_TELEMETRY_DISABLED=1

RUN npm install -g pnpm@11.8.0

WORKDIR /app

COPY pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/package.json
RUN pnpm install --frozen-lockfile

FROM node-deps AS frontend-builder

COPY . .
RUN pnpm --dir apps/web build:web

FROM golang:1.25-alpine AS go-builder

RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
COPY queries ./queries
COPY web ./web
COPY sqlc.yaml ./
COPY --from=frontend-builder /app/web/static ./web/static

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/propulse ./cmd/propulse

FROM alpine:3.22 AS runner

RUN apk add --no-cache \
  ca-certificates \
  chromium \
  freetype \
  harfbuzz \
  nss \
  ttf-freefont

COPY --from=go-builder /out/propulse /usr/local/bin/propulse

EXPOSE 8080

CMD ["/usr/local/bin/propulse", "serve"]
