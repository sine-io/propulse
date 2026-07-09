FROM node:22-alpine AS node-deps

ENV NEXT_TELEMETRY_DISABLED=1

RUN npm install -g pnpm@11.8.0

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

FROM node-deps AS frontend-builder

COPY . .
RUN pnpm build:web

FROM golang:1.25-alpine AS go-builder

RUN apk add --no-cache git

WORKDIR /src

COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download

WORKDIR /src
COPY backend ./backend
COPY --from=frontend-builder /app/backend/web/static ./backend/web/static

WORKDIR /src/backend
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
