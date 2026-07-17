# 房见双小区完整采集

Propulse 的房见采集器仅访问 HAR 中已经验证、与小区市场有关的接口。目标小区为：

| 小区 | 房见小区 ID | 板块编码 |
| --- | --- | --- |
| 富力津门湖鸣泉花园（鸣泉花园） | `a2d56505411446cfe70fd3960beb19c7` | `BK2022112435579` |
| 亲和美园 | `0a5b87b0d81dadbb50fb85df01489a13` | `BK2022112435657` |

采集范围包括城市汇总、地图聚合、小区档案、14 类分析接口、当前挂牌、历史成交、逐套调价，以及竞品、竞品汇总和 POI 周边数据。不会访问微信登录、订阅状态、搜索历史、活动或提示接口。

## 凭证

采集前必须在进程环境中提供：

```bash
export FANGJIAN_AUTHORIZATION='...'
export FANGJIAN_AK='...'
export FANGJIAN_VERSION='...'
```

采集器不会从 HAR、源码、参数文件或默认值读取凭证，也不会把请求头写入日志或归档。凭证失效时会立即失败，不尝试登录、刷新或绕过鉴权。

## 采集

采集两个小区：

```bash
make collect-fangjian
```

也可以只采一个小区：

```bash
go run ./cmd/fangjian-collector --community mingquan --output data/fangjian
go run ./cmd/fangjian-collector --community qinhe --output data/fangjian
```

每次采集写入 `data/fangjian/<UTC时间>/<小区>/`：

- `raw/*.json`：允许范围内各接口的响应体，不含请求头。
- `bundle.json`：可导入的 `fangjian.bundle/v1` 规范化数据包。
- `manifest.json`：来源时间、接口路径与文件清单。
- `SHA256SUMS`：归档文件校验和。
- `result.json`：完整性状态与挂牌、成交、调价数量。

HTTP 超时、429 和 5xx 最多尝试三次；鉴权失败、业务码异常或任一接口失败时不会生成最终归档。

## 100 条上限

挂牌和成交接口先请求全量。返回恰好 100 条时，采集器按户型拆分，并按 `roomId` 合并去重；某户型仍为 100 条时继续按高、中、低楼层拆分。任一叶子仍返回 100 条会以 `fangjian_collection_incomplete` 终止，残缺结果不会归档或导入。

采集器会对挂牌与成交中所有 `adjustNum > 0` 的唯一 `roomId` 请求调价历史。挂牌天数按采集日期与首次挂牌日期计算，户型规范化为“一室、二室、三室”等。

## 导入

管理员接口一次事务写入 `collection_run`、行情快照、挂牌 observation、成交 observation 和调价记录：

```text
POST /admin/api/community-market/imports/fangjian
Authorization: Bearer <PROPULSE_ACCESS_TOKEN>
Content-Type: application/json
```

请求体：

```json
{
  "dataSourceId": "已有数据源 UUID",
  "neighborhoodId": "已有小区 UUID",
  "sourceRef": "fangjian-mingquan-20260717T010203Z",
  "bundle": {}
}
```

`bundle` 使用归档中的 `bundle.json` 内容。相同数据源、来源引用和规范化内容会返回 HTTP 200 与 `idempotentReplay: true`；新数据返回 HTTP 201。质量不是 `complete`、带完整性告警或语义校验失败的数据包返回 422，不写入任何记录。

旧 `POST /admin/api/community-market/imports/csv` 继续兼容既有 34/57 列聚合快照，但这些快照标记为 `aggregate_only`，不会伪装成完整挂牌与成交数据。

## 数据边界

“完整”只表示上述已验证接口在采集时可返回的完整数据。房见未提供的房源详情、户型图和图片不会被生成或补写。原始 HAR 与本地 `data/fangjian/` 均由 `.gitignore` 排除，不应提交到 Git。
