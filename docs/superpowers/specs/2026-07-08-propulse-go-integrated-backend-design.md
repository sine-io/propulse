# Propulse Go 一体化后端技术栈设计

## 背景

Propulse 当前是 `Next.js + React + TypeScript + Tailwind` 前端原型，核心决策逻辑位于前端 `src/lib/decision.ts`，数据来自本地 sample data。下一阶段需要引入完整后端能力，包括账号、业务 API、房源与小区数据采集、清洗、指标计算、提醒、后台管理和持久化存储。

用户明确希望：

- 后端使用 Go。
- 前端 React 保持不变，但嵌入 Go 后端。
- 一个 `go build` 产出一个可启动完整服务的二进制。
- 同时支持 Docker Compose 启动。
- 日志使用 `zerolog`。
- 架构遵循 DDD Lite + CQRS + Clean Architecture + DIP。
- 第一版按完整产品后端设计，包括自动采集、人工导入/修正和未来 API 数据源扩展。

## 推荐结论

采用：

> Go + Gin + PostgreSQL + Redis/Asynq + GORM/sqlc + golang-migrate + chromedp + zerolog + OpenAPI + Next static export + go:embed。

部署形态采用：

> 单仓库、单 Go binary、多启动模式、可单进程运行、可 Docker Compose 拆进程运行。

这套方案兼顾：

- 一个二进制启动完整产品。
- 前端不需要独立 Node 服务。
- 早期开发和部署简单。
- 后期可把 API、Worker、Scheduler 拆成多个进程。
- 数据采集、清洗、指标计算和产品 API 有清晰边界。

## 总体架构

```text
propulse binary
  ├─ web server
  │   ├─ embedded React/Next static frontend
  │   ├─ REST API
  │   └─ admin API
  ├─ scheduler
  │   └─ enqueue collection / metric / notification jobs
  ├─ worker
  │   ├─ listing collection
  │   ├─ data normalization
  │   ├─ deduplication
  │   ├─ metric calculation
  │   └─ notification generation
  └─ migration command
```

外部依赖：

```text
PostgreSQL  # 业务数据、房源快照、小区指标、用户数据
Redis       # Asynq 队列、缓存、任务锁
Chromium    # 动态网页采集，供 chromedp 驱动
```

## 启动模式

同一个 Go binary 支持多种命令：

```bash
propulse serve        # API + frontend + scheduler + worker，全量启动
propulse api          # 只启动 HTTP API 和嵌入前端
propulse worker       # 只启动异步任务 worker
propulse scheduler    # 只启动定时任务调度器
propulse migrate up   # 执行数据库迁移
propulse migrate down # 回滚数据库迁移
```

本地和早期生产环境可以使用：

```bash
propulse serve
```

当采集或指标任务变重时，Docker Compose 可以拆进程运行同一个 binary：

```yaml
services:
  app:
    image: propulse:local
    command: ["propulse", "api"]

  worker:
    image: propulse:local
    command: ["propulse", "worker"]

  scheduler:
    image: propulse:local
    command: ["propulse", "scheduler"]

  postgres:
    image: postgres:16

  redis:
    image: redis:7
```

## 前端嵌入方案

当前 React/Next 前端保持不变，但需要按静态站点导出：

```js
// next.config.mjs
const nextConfig = {
  output: "export",
};

export default nextConfig;
```

构建流程：

```bash
pnpm build
```

输出：

```text
out/
```

Go 后端通过 `go:embed` 嵌入静态产物：

```go
//go:embed web/*
var webFS embed.FS
```

HTTP 路由约定：

```text
/             -> embedded frontend
/api/v1/*     -> product REST API
/admin/api/*  -> admin REST API
/healthz      -> liveness check
/readyz       -> readiness check
/metrics      -> optional metrics endpoint
```

限制：

- 前端不能依赖 Next SSR。
- 不使用 Next Server Actions。
- 不使用 Next API Routes 承担后端逻辑。
- 动态数据统一通过 Go REST API 获取。

## 架构原则

### DDD Lite

使用领域边界组织业务，但避免过度建模。

核心领域：

- `auth`：认证、会话、权限。
- `user`：用户资料、家庭画像。
- `capacity`：购房/换房能力测算。
- `neighborhood`：小区档案、观察池、目标小区。
- `listing`：房源、挂牌、成交、降价记录。
- `collection`：数据源、采集任务、采集结果。
- `decision`：预算压力、小区信号、出手窗口。
- `notification`：提醒、周报、异常信号。
- `admin`：后台配置、人工修正、任务观察。

DDD Lite 约束：

- 领域对象只表达业务规则，不直接依赖 HTTP、数据库、Redis、Asynq。
- 聚合保持小而实用，不为所有表强行建聚合。
- 领域服务只放跨实体的核心判断逻辑。
- 读模型可以为页面和报表服务，不必强行复用写模型。

### CQRS

写入路径和读取路径分离。

Command 侧负责：

- 创建目标小区。
- 添加观察项。
- 保存测算记录。
- 创建采集任务。
- 人工修正房源/小区数据。
- 确认或忽略提醒。

Query 侧负责：

- 首页决策摘要。
- 换房测算详情。
- 小区观察看板。
- 出手窗口结果。
- 后台采集任务列表。
- 小区指标趋势。

约束：

- Command handler 返回操作结果，不承担复杂页面组装。
- Query handler 可以面向 UI 返回专用 DTO。
- 指标、历史趋势、后台列表优先使用 sqlc / 原生 SQL。
- 简单后台 CRUD 可以使用 GORM。

### Clean Architecture

依赖方向：

```text
interfaces/http
  -> application
    -> domain
    <- infrastructure
```

层次职责：

- `domain`：实体、值对象、领域服务、领域错误。
- `application`：command/query handler、用例编排、事务边界。
- `infrastructure`：PostgreSQL、Redis、Asynq、chromedp、外部数据源。
- `interfaces`：HTTP handler、middleware、OpenAPI DTO、静态前端服务。

规则：

- `domain` 不 import `gorm`、`sqlc`、`gin`、`asynq`、`redis`、`chromedp`。
- `application` 依赖 repository/interface，不依赖具体数据库实现。
- `infrastructure` 实现 application/domain 定义的接口。
- `interfaces/http` 只做协议适配、鉴权、参数校验和响应映射。

### DIP

依赖倒置通过接口约束边界。

示例：

```text
application/decision
  depends on NeighborhoodMetricReader interface

infrastructure/postgres
  implements NeighborhoodMetricReader
```

典型接口：

- `UserRepository`
- `NeighborhoodRepository`
- `ListingSnapshotWriter`
- `NeighborhoodMetricReader`
- `CollectionJobQueue`
- `BrowserCollector`
- `NotificationSender`
- `Clock`
- `TransactionManager`

这样可以做到：

- 应用层不关心数据来自 GORM、sqlc 还是外部 API。
- 采集逻辑可以替换普通 HTTP、chromedp 或未来授权 API。
- 测试时可以用内存 fake 实现用例验证。

## 数据访问策略

采用 GORM + sqlc 混合模式。

### 使用 GORM 的场景

- 用户。
- 账号权限。
- 后台配置。
- 数据源配置。
- 内容模板。
- 简单管理 CRUD。

理由：

- 早期开发快。
- 后台表结构变化频繁。
- CRUD 逻辑多，复杂统计少。

### 使用 sqlc / 原生 SQL 的场景

- 房源快照。
- 小区指标趋势。
- 挂牌/成交/降价聚合。
- 数据去重。
- 指标计算读取。
- 周报和后台报表。

理由：

- 查询复杂。
- 时间序列和聚合多。
- 需要明确 SQL 性能和索引策略。
- 类型安全比手写扫描更稳。

## 任务系统

使用 Redis + Asynq。

任务类型：

- `collection.fetch_source`
- `collection.normalize_listing`
- `collection.deduplicate_listing`
- `metric.calculate_neighborhood`
- `decision.refresh_window`
- `notification.generate_weekly_report`
- `notification.send_alert`

任务原则：

- 任务 payload 只放 ID 和必要参数，不放大对象。
- 任务 handler 幂等。
- 采集任务必须有数据源级限速。
- 失败任务进入重试和死信观察。
- 指标计算和提醒生成通过任务串联，不在 HTTP 请求中同步执行。

## 数据采集设计

采集采用混合数据源策略：

- 公开网页采集。
- 后台 CSV/Excel 导入。
- 后台人工修正。
- 未来授权 API 或付费数据源接入。

全部采集逻辑仍使用 Go 实现。

采集能力分两层：

```text
普通 HTTP collector
  -> 适合静态页面、JSON endpoint、低成本数据源

chromedp browser collector
  -> 适合动态渲染页面、需要浏览器环境的数据源
```

采集结果不直接覆盖业务表，而是先进入原始数据和快照层：

```text
raw_collection_records
  -> normalized_listings
    -> listing_snapshots
      -> neighborhood_metrics
        -> decision_results
```

这样可以：

- 保留原始证据。
- 支持重新清洗。
- 避免采集异常污染核心业务数据。
- 允许后台人工修正后重新计算指标。

## 日志与可观测性

日志使用 `zerolog`。

日志要求：

- 全部结构化 JSON。
- 每个 HTTP 请求带 `request_id`。
- 每个任务带 `job_id`、`task_type`、`source_id`。
- 采集链路带 `collection_run_id`。
- 用户相关操作记录 `user_id`，避免记录敏感明文。
- 错误日志包含稳定错误码和可定位上下文。

日志分层：

```text
interfaces/http    request log, response status, latency
application        use case start/end, domain decision summary
infrastructure     db/redis/asynq/browser/data source failures
worker             job lifecycle, retry, dead letter
scheduler          scheduled job enqueue result
```

## API 设计

采用 REST + OpenAPI。

推荐路径：

```text
POST /api/v1/auth/login
GET  /api/v1/me

POST /api/v1/capacity/calculations
GET  /api/v1/capacity/calculations/:id

POST /api/v1/neighborhoods
GET  /api/v1/neighborhoods/:id
GET  /api/v1/neighborhoods/:id/metrics

POST /api/v1/watchlist/items
GET  /api/v1/watchlist

GET  /api/v1/decision/action-window

GET  /admin/api/collection-runs
POST /admin/api/data-sources
POST /admin/api/imports
POST /admin/api/corrections
```

前端通过 OpenAPI 生成 TypeScript 类型和 API client，减少字段漂移。

## 推荐目录结构

```text
backend/
  cmd/
    propulse/
      main.go

  internal/
    domain/
      auth/
      user/
      capacity/
      neighborhood/
      listing/
      collection/
      decision/
      notification/

    application/
      auth/
      capacity/
      neighborhood/
      listing/
      collection/
      decision/
      notification/

    interfaces/
      http/
        handler/
        middleware/
        dto/
        router/
      web/

    infrastructure/
      postgres/
        gorm/
        sqlc/
      redis/
      queue/
      browser/
      datasource/
      logger/
      config/
      migrate/

    platform/
      app/
      clock/
      transaction/
      errors/

  migrations/
  queries/
  web/
    .gitkeep
```

根目录可以继续保留当前前端：

```text
src/
package.json
next.config.mjs
```

构建时将 `out/` 复制或同步到：

```text
backend/web/
```

再由 Go embed。

## Docker Compose 设计

Compose 至少包含：

```text
propulse
postgres
redis
```

如果 browser collector 需要独立环境，可以扩展：

```text
propulse-worker
chromium dependencies in same image
```

第一版不建议把浏览器采集拆成独立语言服务，因为用户已明确要求全部后端使用 Go。

## 测试策略

测试分层：

- `domain`：纯单元测试，覆盖预算压力、小区信号、出手窗口规则。
- `application`：用 fake repository 测 command/query handler。
- `infrastructure/postgres`：用测试数据库验证迁移、sqlc 查询、索引关键路径。
- `interfaces/http`：用 httptest 验证 API 状态码、鉴权、DTO 映射。
- `worker`：验证任务幂等、重试和失败处理。
- `frontend`：保留现有 Vitest 和 React Testing Library。

## 主要风险与取舍

### Next 静态导出限制

嵌入 Go binary 后，不能依赖 Next SSR。当前项目以页面展示和客户端交互为主，可以接受。后续动态数据统一从 Go API 获取。

### 全 Go 采集复杂度

Go 生态可以通过 HTTP client 和 chromedp 实现采集，但动态网页采集开发效率通常低于 Node Playwright。为保持技术栈统一，第一版接受这项复杂度，并通过 collector interface 隔离实现。

### 单 binary 不等于零依赖

PostgreSQL 和 Redis 仍是必要依赖。考虑到房源快照、时间序列指标、任务队列和提醒，SQLite 与进程内队列不适合作为正式产品主方案。

## 第一阶段落地范围

第一阶段建议只落以下基础设施和一个端到端闭环：

1. Go 项目骨架。
2. 嵌入当前 Next static export。
3. Gin HTTP server。
4. zerolog request/job logging。
5. PostgreSQL + migration。
6. Redis + Asynq。
7. DDD Lite + CQRS + Clean Architecture 目录边界。
8. 换房测算 API。
9. 目标小区 watchlist API。
10. 一个模拟或手动导入的数据源。
11. 一个小区指标计算任务。
12. 前端从 Go API 读取真实数据。

这能验证：

- 一个二进制启动前后端。
- API、数据库、队列、Worker 跑通。
- 当前前端能从后端取数据。
- 后续采集和指标系统有清晰扩展位置。
