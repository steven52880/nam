# M5 定价计划

依据 `docs/requirements.md` 第 25 轮（本计划同时定稿）。分两批，每批编译通过、主人在浏览器试过再提交。

## 需求结论（第 25 轮）

- **售价单位**：原厂价的百分比，内部存小数（0.30 = 30%）。
    - 厂商分组倍率 = 售价 × 美元参考汇率 ÷ 站点汇率，写入时四舍五入到 **3 位小数**（第 26 轮由 6 位改为 3 位）。接管时已有倍率不是 3 位小数也判为已同步（同步容差 = 半个写入步长 5e-4）。
    - 固定模式下可直接编辑倍率：`PUT /vendor-pricing/:id {"group_ratio": 0.309}`，服务端取整到 3 位后反算 `sell_price = 倍率 × 站点汇率 ÷ 美元参考汇率`；跟随模式下返回 400；不能与 `sell_price` 同时给。
    - 价格库只匹配原厂（见 `requirements.md` 第 26 轮）；模型表去掉偏差列。以下契约示例里的 6 位小数倍率仅为旧值，按 3 位理解。
    - 界面同时显示折合人民币："≈ 2.16 元 / 原厂 $1"（= 售价 × 美元参考汇率）。
    - 改美元参考汇率后各厂商的目标倍率会变，按"待写入"处理（固定模式也要手动写入）。
- **参考成本**（跟随模式用），只用当前配置，不查日志：
    - 候选：该厂商分组下（NewAPI **已生效**的 group = 厂商分组名，草稿不算）、状态为启用、且有上游渠道售价的渠道。
    - 取候选中 **priority 最大** 的那一层。
    - 按 NewAPI 的 weight 加权平均它们的**上游渠道售价**（渠道级，模型级倍率覆盖不计）；该层权重全为 0 时取简单平均。
    - 没有候选渠道 → 状态"无可用渠道"，不写入。
- **跟随模式**：目标售价 = 参考成本 × 加价比例，四舍五入到 4 位小数（0.01%）。
    - 变化 = |目标售价 − 上次写入售价| ÷ 上次写入售价。
    - 变化 > 变化保护阈值（默认 50%）→ "待确认"，记下 `pending_price`，不写。主人确认后写入。
    - 变化 < 最小变动（默认 2%）→ 定时任务不写（手动"预览写入"仍会包含）。
    - 定时任务**只自动写跟随模式**；固定模式的改动一律手动写入。
- **自动写入**：新设置键 `pricing_auto_write`，默认 `false`；旧键 `auto_write` 废弃（从 `settingKeys`、`Defaults` 删除）。定价任务默认每 1 小时。
- **首次接管**：厂商还没有 `vendor_pricing` 行时自动建一行：
    - NewAPI `GroupRatio` 里有该分组 → 固定模式，售价 = 现有倍率 × 站点汇率 ÷ 美元参考汇率，`last_written_ratio` = 现有倍率，`last_written_price` = 该售价。<u>保证不改价</u>。
    - 没有 → 固定模式、售价为空，状态"未设置"。
- **漂移**：NewAPI 当前倍率与 `last_written_ratio` 相差 > 1e-6 → "漂移"，自动写入跳过。操作：
    - 采纳 NewAPI 值：转固定模式，售价按 NewAPI 倍率反算，只写 nam。
    - 以 nam 为准覆盖：`apply` 带 `force`，只对指定厂商生效。
- **写 NewAPI 的范围**（只读改这三项；第二批再加 `Price`、`USDExchangeRate` 和模型价格）：
    - `GroupRatio`（JSON 对象）：只改厂商分组的键，其它键原样保留。
    - `UserUsableGroups`（JSON 对象 分组→描述）：补上各厂商分组（描述缺失时用厂商名）和 `auto`（缺失时描述"自动选择"），已有项不动。
    - `AutoGroups`（JSON 数组）：先按 nam 厂商顺序列出厂商分组，再接原有的其它项（保持原相对顺序、去重）。
    - 删除厂商<u>不会</u>删除 NewAPI 里的分组键（已知限制）。
- **回滚**（第二批）：恢复的是"售价"；跟随模式的厂商回滚后自动改成固定模式。
- **通知**："待确认""漂移"只在界面显示（总览告警 + 导航徽标），推送等 M6。
- **毛利率**：厂商行显示近 7 天毛利率 = (收入 − 成本) ÷ 收入，来自 `stats_hourly`（按 `vendor` 汇总 `revenue_cny`、`cost_cny`）。

## 分批

- **第一批**：厂商定价（计算、首次接管、编辑、预览写入、确认、采纳）、定价任务、写入记录 `price_history`、设置项、模型与价格页的厂商定价区、预览写入弹窗、设置页定价参数、总览告警和导航徽标。
- **第二批**：站点汇率修改（二次验证，同时写 `Price`、`USDExchangeRate` 并重写所有厂商倍率，不触发变化保护）；写入历史列表与回滚；"按建议执行"：勾选模型把原厂价写进 NewAPI 的模型价格（先预览）；"NewAPI 未设价"状态。

## 后端架构（第一批）

### 新包 `nam/internal/vendorprice`

- `compute.go`（纯函数，重点单测）：
    - `ReferenceCost(chs []ChannelInput) (*Reference)`：`ChannelInput{ID, Name, Status, Priority, Weight, ChannelCost *float64}`；返回 `Reference{Cost, Priority, Channels []RefChannel}` 或 nil。
    - `TargetSellPrice(mode, sellPrice, markup *float64, ref *Reference) *float64`。
    - `GroupRatio(sell, usdRate, siteRate float64) (float64, error)`：任一非有限或 ≤ 0 时报错。
    - `Classify(...) State`：按 未设置 → 无可用渠道 → 漂移 → 待确认 → 待写入 → 已同步 的优先级判定。
- `options.go`：三个选项的解析（非法 JSON 直接报错，**不覆盖**）、合并、序列化、比较（按解析后的值比较）。
- `service.go`：`Service{St *store.Store, Options ports.OptionStore, Channels *channels.Service, Audit *audit.Recorder}`，一个互斥锁保护写入：
    - `Overview(ctx)`、`Update(ctx, id, in)`、`Preview(ctx, in)`、`Apply(ctx, in, trigger)`、`Confirm(ctx, id)`、`Adopt(ctx, id)`、`Job(ctx)`。
    - `Overview` 内做首次接管（创建缺失的 `vendor_pricing` 行）。
- 写入流程（`Apply`/`Confirm` 共用）：
    1. 读三个选项并解析；任一解析失败 → 返回错误，不写。
    2. 计算新值，校验所有倍率有限且 > 0。
    3. 只写变化的键，顺序：`GroupRatio` → `UserUsableGroups` → `AutoGroups`（经审计包装的 `OptionStore.Set`）。
    4. 读回，按解析后的值核对；不一致时返回错误并记一条失败审计。
    5. 每个改了倍率的厂商记一条 `price_history`，更新 `vendor_pricing`（`last_written_price`、`last_written_ratio`、`last_written_at`、`state`、清空 `pending_price`）。

### 其它改动

- `store/models.go`：
    - `VendorPricing`：`SellPrice`、`Markup` 改为 `*float64`（语义：售价是原厂价倍数）；`LastWritten` 改名 `LastWrittenPrice`（列名可保留，按 gorm 能迁移的方式改）；新增 `LastWrittenRatio *float64`、`LastWrittenAt int64`。状态常量：`synced`、`to_write`、`pending`、`drift`、`no_channel`、`unset`（删掉 `no_price`）。
    - `PriceHistory`：新增 `VendorID *uint`；`Key` 用 `vendor:<id>`；`Before`/`After` 是 JSON `{"mode","sell_price","group_ratio"}`；`Reason`：`manual`/`auto`/`confirm`/`force`。
    - 现有库里如果已有 `vendor_pricing` 行（理论上没有），迁移时按旧语义丢弃即可。
- `store/store.go`：新增 `KeyPricingAutoWrite = "pricing_auto_write"`（默认 `false`）、`KeyPricingMinChange = "pricing_min_change"`（默认 `0.02`）；删除 `KeyAutoWrite`。
- `api/handlers.go`：`settingKeys` 与校验跟着改（`pricing_min_change`：0 ≤ x < 1）。
- `channels`：新增方法返回各渠道 **NewAPI 已生效值**（不叠草稿）：`ID, Name, Group, Status, Priority, Weight, ChannelCost`。
- `stats`：新增按厂商汇总近 N 天 `revenue_cny`、`cost_cny`。
- `core/core.go`：装配 `vendorprice.Service`，`JobPricing` 改用 `Job`，默认间隔 1 小时。
- `api/vendor_pricing.go`：注册下列接口；`docs/api/openapi.yaml` 同步。
- `api/vendors.go`：删掉 `TODO(M5)`（厂商顺序变化体现在 `AutoGroups` 的"待写入"里）。

### 接口契约（前后端都以此为准）

所有接口在 `/api/nam` 下，响应外层沿用现有 `resp.OK` 格式。

`GET /vendor-pricing`
```json
{
  "site_rate": 7, "usd_rate": 7.2, "auto_write": false,
  "protect_threshold": 0.5, "min_change": 0.02,
  "items": [{
    "vendor_id": 1, "vendor": "Anthropic", "group": "claude", "order": 0,
    "mode": "follow",
    "markup": 1.5,
    "sell_price": null,
    "reference": { "cost": 0.2, "priority": 10,
      "channels": [{ "id": 3, "name": "a", "weight": 10, "channel_cost": 0.2 }] },
    "target_price": 0.3,
    "target_ratio": 0.308571,
    "newapi_ratio": 0.2857,
    "last_written_price": 0.2778, "last_written_ratio": 0.2857, "last_written_at": 1730000000,
    "pending_price": null,
    "change": 0.08,
    "small_change": false,
    "state": "to_write",
    "revenue_7d": 120.5, "cost_7d": 80.1, "margin_7d": 0.335
  }],
  "groups": {
    "auto_groups": { "current": ["claude", "default"], "target": ["claude", "openai", "default"] },
    "user_usable_groups": { "current": { "default": "默认" }, "target": { "default": "默认", "claude": "Anthropic", "auto": "自动选择" } },
    "changed": true
  },
  "error": ""
}
```
- 数值字段可为 `null`；`sell_price` 只在固定模式有意义；`markup` 只在跟随模式有意义。
- `error` 非空表示 NewAPI 选项解析失败（此时不能写入）。

`PUT /vendor-pricing/:id`，请求 `{ "mode": "fixed"|"follow", "sell_price": 0.3, "markup": 1.5 }`（缺省字段不改）→ 返回单个 item。只写 nam；会清空 `pending_price`。校验：售价 0 < x ≤ 10，加价比例 0 < x ≤ 100。

`POST /vendor-pricing/preview` 与 `POST /vendor-pricing/apply`，请求相同：
```json
{ "vendor_ids": [1, 2], "include_groups": true, "force": false }
```
- `vendor_ids` 省略 = 所有"待写入"的厂商（含 `small_change`）；`待确认`、`未设置`、`无可用渠道` 一律跳过；`漂移` 只有 `force: true` 且在 `vendor_ids` 里明确列出时才写。
- `include_groups` 省略 = true。
- 服务端重新计算，不信任客户端传的数值。

preview 响应：
```json
{
  "vendors": [{ "vendor_id": 1, "vendor": "Anthropic", "group": "claude",
    "before_price": 0.2778, "after_price": 0.3, "before_ratio": 0.2857, "after_ratio": 0.308571 }],
  "skipped": [{ "vendor_id": 2, "vendor": "OpenAI", "state": "pending", "reason": "变化 +80%，超过变化保护阈值" }],
  "options": [{ "key": "GroupRatio", "before": "{...}", "after": "{...}" }]
}
```
apply 响应：`{ "written": ["GroupRatio", "AutoGroups"], "vendors": [...同 preview...], "skipped": [...] }`；失败时 HTTP 错误，消息说明在哪一步失败。

`POST /vendor-pricing/:id/confirm`：写入 `pending_price`（只对 `pending` 状态），响应同 apply。

`POST /vendor-pricing/:id/adopt`：只对 `drift` 状态，响应单个 item。

`POST /jobs/pricing`：已有的任务触发接口，执行 `Job`。

### 定时任务 `Job`

1. 计算全部厂商（含首次接管），把"待确认"状态和 `pending_price` 落库；变化回到阈值内时清掉待确认。
2. `pricing_auto_write` 为 true 时：对跟随模式、状态"待写入"且不是 `small_change` 的厂商执行 apply（`include_groups: true`），触发方式记 `auto`。
3. 返回摘要：`N 个厂商；写入 x；待确认 y；漂移 z；无可用渠道 w`。

## 前端（第一批）

- **模型与价格页顶部"厂商定价"区**（`web/src/pages/pricing/` 下新文件，表格用 `DataTable`）：
    - 列：厂商（含分组名）、模式 ✎、加价比例 ✎（仅跟随）、参考成本（%，悬停列出所用渠道、优先级、权重）、售价 ✎（固定可编辑；跟随为只读计算值；旁边小字 ≈ 元/原厂 $1）、NewAPI 当前倍率 → 目标倍率、变化、近 7 天毛利率、状态。
    - 行操作：待确认 →"确认"；漂移 →"采纳 NewAPI 值" / "以 nam 为准覆盖"（二次确认）。
    - 编辑即调 `PUT`，只写 nam，状态变成"待写入"。
- 页头按钮："立即计算"（触发 `POST /jobs/pricing`）和"预览写入"去掉禁用占位；删掉"售价与分组倍率在 M5 实现"提示。
- **预览写入弹窗**：列出厂商倍率变化、跳过的厂商及原因、三个选项的变化（JSON 前后对比，可折叠）；确认后调 `apply`。
- **设置页"定价参数"**：变化保护阈值、最小变动、自动写入开关（键 `pricing_auto_write`）；美元参考汇率提示加一句"修改后各厂商分组倍率需要重新写入"。删掉"冷启动策略"。
- **总览告警 + 导航"模型与价格"徽标**：待确认 + 漂移的厂商数。
- 文案：参数名用中文（售价、参考成本、加价比例、厂商分组倍率、变化保护阈值、最小变动）；百分比用 `formatRatioPercent`。

## 验证

- 后端：`go vet ./...`、`go test ./...`、`go build -o nam.exe ./cmd/nam`。
    - `vendorprice` 单测：参考成本（最高层、加权、权重全 0、无候选）、目标售价、倍率公式、状态判定、选项合并（保留其它分组、AutoGroups 顺序、UserUsableGroups 补 auto、非法 JSON 不写）、首次接管不改价、写入后读回、`price_history` 记录、自动任务只写跟随模式且跳过小变动。
    - 接口测试：`core/m5_test.go`，用 `portstest` 的假适配器跑计划 §11.1 M5 第 2、3、5 步的等价场景（数值按新口径换算）。
- 前端：`npm.cmd run build`、`npm.cmd test`。
- 主人在浏览器试用后提交。
