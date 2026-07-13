# 房脉 / propulse

房脉（propulse）是一个面向刚需与改善用户的 **换房决策助手**。

它不做玄学房价预测，也不做泛房产资讯。它解决的是一个更具体的问题：

> 我想买房或换房，但不知道现在能不能买、买哪里、压力多大、什么时候出手。

## 核心价值

房脉帮助用户：

- 算清自己的购房能力与换房压力
- 监测目标小区的挂牌、成交、降价与供应信号
- 判断当前更适合看房、等待、砍价还是出手
- 理解每个判断背后的房产方法论
- 建立一套可复盘、可迁移的换房决策系统

## 你可以把它理解成

- 一个换房能力计算器
- 一个目标小区观察台
- 一个买方窗口判断器
- 一个带解释的房产决策工具
- 一个让用户边决策边学习的方法论系统

## 不做什么

房脉不试图成为：

- 全国房价预测平台
- 房产资讯流
- 房产培训课
- 中介导流站
- 黑箱式买卖建议工具

## 核心模块

1. **换房测算**：算清安全总价、月供压力、资金缺口和风险边界。
2. **目标小区**：跟踪用户关注小区的价格、供应、成交与降价信号。
3. **出手窗口**：给出“看 / 等 / 砍价 / 出手”的阶段性判断。
4. **判断方法**：解释为什么这么判断，帮助用户形成自己的大局观。
5. **工具模板**：提供监测表、预算表、周复盘表和谈价清单。

## 核心原则

1. 先解决决策焦虑，再讲方法论。
2. 不预测涨跌，只判断信号和压力。
3. 不替用户拍板，但必须给出明确建议。
4. 每个结论都要能解释原因。
5. 小区级、预算级、家庭现金流级，比宏观大盘更重要。
6. 用户每次使用，都应该更会判断。

## 项目文档

- [产品简介](docs/product-overview.md)
- [信息架构](docs/information-architecture.md)
- [页面线框](docs/wireframes.md)
- [PRD 第一版](docs/prd-v1.md)

## 本地开发与验证

安装前端依赖并运行完整前端校验：

```bash
pnpm --dir apps/web install --frozen-lockfile
pnpm --dir apps/web verify
```

修改前端后，刷新 Go `embed` 使用的静态产物：

```bash
pnpm --dir apps/web build:web
```

仓库会跟踪 `apps/web/embed/static` 中的导出结果，因此干净检出无需安装 Node.js 依赖即可构建包含前端的 Go 二进制：

```bash
go build ./cmd/propulse
```

运行后端测试：

```bash
go test ./...
```

使用 Docker Compose 启动集成服务：

```bash
docker compose up --build
```

服务启动后可检查健康状态和关注列表 API：

```bash
curl http://127.0.0.1:8317/healthz
curl http://127.0.0.1:8317/api/v1/watchlist
```

Compose 会为本地管理接口设置 `PROPULSE_ADMIN_API_TOKEN=local-admin-token`。调用 `/admin/api/*` 时需要传入 bearer token：

```bash
curl -X POST http://127.0.0.1:8317/admin/api/imports \
  -H "Authorization: Bearer local-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"sourceType":"manual_json","sourceRef":"demo","neighborhoodId":"<uuid>","records":[{"listingPrice":520,"daysOnMarket":30}]}'
```

也可以运行本地集成冒烟脚本，完成 Compose 构建、健康检查、关注列表 API 和 E2E smoke test：

```bash
bash scripts/verify-stack.sh
```

## 后端二进制模式

同一个 `propulse` Go 二进制支持以下运行模式：

```bash
propulse serve
propulse api
propulse worker
propulse scheduler
propulse migrate up
propulse migrate down
```

## 建议下一步

先看：

1. `docs/product-overview.md`
2. `docs/prd-v1.md`
3. `docs/information-architecture.md`
