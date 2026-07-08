FROM node:22-alpine AS base

ENV NEXT_TELEMETRY_DISABLED=1

RUN apk add --no-cache libc6-compat \
  && npm install -g pnpm@11.8.0

WORKDIR /app

FROM base AS deps

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

FROM base AS builder

COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN pnpm build

FROM base AS runner

ENV NODE_ENV=production

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml next.config.mjs ./
RUN pnpm install --prod --frozen-lockfile \
  && pnpm store prune

COPY --from=builder /app/.next ./.next

EXPOSE 3000

CMD ["pnpm", "exec", "next", "start", "-H", "0.0.0.0", "-p", "3000"]
