# M4 统计：实施计划

依据：`newapi-manager-plan.md` §7、§11.1 M4；`docs/requirements.md` T1–T5；`docs/ui-features.md` §4.2、§6、§10。本文件与它们冲突时，以本文件为准（下面列了改动点）。

## 主人已决定（第 21 轮）

- **统计维度**：按渠道统计（一个渠道 = 一个上游令牌），再按模型、上游、厂商汇总。
- **补丁 P1**：要做，放在 clansty 仓库 `nam-patches` 分支。<u>没打补丁的 NewAPI 也必须能用</u>：吐字速度退回按秒估算，界面标「估」。
- **错误日志**：不确定有没有开 `ERROR_LOG_ENABLED`。最近 7 天有成功请求却没有一条错误日志时，统计页和渠道页提示「可能没开错误日志」。
- **清理 NewAPI 原始日志**：默认关闭，在设置里手动打开。任何情况下都不删还没拉取的日志。
- **图表**：recharts，统计页懒加载，首页包体积不变。
- **首次回填天数**：做成设置项（默认 7 天）；另有「清空并重新统计」按钮，可以选天数。
- **成本价两种都算**：
    - 定价口径 `measured_cost_price` = 实际成本 ÷ 按原厂缓存价算的原厂成本（M5 定价用）。
    - 有效口径 `effective_cost_price` = 实际成本 ÷ 不算缓存时的原厂成本（缓存命中越高越低）。
    - 渠道表默认显示：渠道倍率（已有的「上游渠道成本价」列）、缓存命中率、实测成本（= 有效口径）。定价口径和标准负载成本价放在隐藏列。
    - **第 22 轮主人确认**：实测成本 = 有效口径，缓存率越高值越低，用来在渠道之间比价。
- **统计页**：按 `ui-features.md` §6 一次做完。

## 相对原计划的改动

- P1 只加 `other.use_time_ms`，**不改 `frt`**（原计划要把非流式的 `-1000` 改成不写）。nam 本来就忽略 `frt ≤ 0`，少改一处，NewAPI 自己的界面也不受影响。
- 原始日志清理新增开关 `newapi_log_cleanup`（默认 `false`）。旧键 `log_retention_days` 只表示天数。不改旧键的默认值，因为已有的 `nam.db` 里已经存了 30。
- 标准负载成本价按渠道的测试模型计价（与「单次成本」选模型的规则相同），token 构成取该厂商最近 7 天全站的构成。渠道没有缓存价覆盖时，它就等于「折扣 × 汇率」，这是原计划定义的自然结果。
- 质量档在请求数少于 20 时不评（显示「样本不足」），否则 10 次全成功也会因为 Wilson 下界 0.72 被评成「差」。
- 价格档：同厂商里有标准负载成本价的渠道少于 2 个时不评。

## 数据口径（所有任务共用，不得各自另定）

### 单条日志怎么解析（`stats.Parse`）

- **只处理** type=2（消费，算成功）和 type=5（错误，算失败）。`token_name == "模型测试"` 的跳过；`channel` ≤ 0 的跳过。
- **token 归一**（`other` 是 JSON 字符串，字段缺失按 0）：
    - 缓存读 `read = other.cache_tokens`
    - 缓存写：`w5 = other.cache_creation_tokens_5m`，`w1 = other.cache_creation_tokens_1h`，`rest = max(0, other.cache_creation_tokens − w5 − w1)` 并入 `w5`；写入总量 `write = w5 + w1`
    - 非缓存输入：`other.usage_semantic == "anthropic"` 时 `input = prompt_tokens`（Anthropic 的 prompt 本来就不含缓存）；否则 `input = max(0, prompt_tokens − read − write)`
    - 输出 `output = completion_tokens`
- **首字延迟**：只取 `is_stream == true` 且 `other.frt > 0`（毫秒）。非流式的 `-1000` 和 0 都丢掉。
- **吐字速度**（token/秒）：只取流式、`completion_tokens > 0`、`frt > 0`。
    - 有 `other.use_time_ms`（补丁 P1）：`speed = completion ÷ ((use_time_ms − frt) / 1000)`，精确。
    - 没有：`speed = completion ÷ ((use_time × 1000 − frt) / 1000)`，**标为估算**。分母 ≤ 0 时丢掉。
- **错误类别**（type=5，看 `other.status_code` 和 `content` 小写后），按顺序命中第一个：
    - `timeout`：状态码 408/504，或含 `timeout`、`deadline exceeded`、`超时`
    - `429`：状态码 429
    - `401`：状态码 401/403
    - `balance`：状态码 402，或含 `insufficient_quota`、`余额不足`、`额度不足`、`insufficient balance`、`credit`
    - `5xx`：状态码 500–599
    - `other`：其余
- **补丁检测**：任何一条消费日志带 `use_time_ms` 时，写 KV `stats_patch_p1_seen=true`。任何一条错误日志入库时，写 KV `stats_error_log_seen_at=<created_at>`。

### 成本与收入（`stats.Pricer`，每次拉取任务开始时构建一次）

- 原厂价：`pricing.Book.Resolve(model_name)`（含别名和手动覆盖）。
- 渠道参数来自 `channel_ext` + `upstream`：折扣 `pricing.EffectiveDiscount(ext.Discount, ModelDiscounts(ext.ModelDiscount), model)`、上游货币汇率 `upstream.CurrencyRate`、缓存覆盖 `ParseCacheRatio(ext.CacheOverride)`。
- 一条成功日志 `u = Usage{Input, CacheRead: read, CacheWrite5m: w5, CacheWrite1h: w1, Output, Requests: 1}`：
    - `official = OfficialUSD(p, u, CacheRatio{})`：按原厂缓存价算的原厂成本（美元）
    - `officialNoCache = OfficialUSD(p, Usage{Input: input+read+write, Output, Requests: 1}, CacheRatio{})`：不算缓存时的原厂成本
    - `cost = ChannelCNY(OfficialUSD(p, u, cr), disc, rate)`：实际成本（元），缓存覆盖只影响这一项
- **只有能算出 cost 的请求**（有原厂价、渠道被 nam 管理且有上游）才把 cost、official、officialNoCache 累加进去，并计入 `priced_requests`。其余只算次数和 token。
- 收入：`quota ÷ QuotaPerUnit × 站点汇率`。QuotaPerUnit 取 `Backend.Info()`，为 0 时用 500000；站点汇率取 KV `site_rate`。所有成功日志都算收入。
- 成本按**拉取时**的折扣和价格计算，之后改折扣不回写历史；要按新价格重算就用「清空并重新统计」。「最新」档每次查询时现算。

### 指标（`stats.Agg.Metrics()`，三档和统计页共用）

- `requests = success + fail`；`success_rate = success ÷ requests`；`wilson` = Wilson 下界（z = 1.96）。requests = 0 时两者都是 null。
- `cache_hit = read ÷ (input + read + write)`，分母为 0 时 null。
- `frt_p50/p95`、`speed_p50/p95` 来自直方图；没有样本为 null。精确样本 ≥ 1 时用精确直方图，`speed_estimated=false`；否则用估算直方图，`speed_estimated=true`。
- `effective_cost_price = cost ÷ officialNoCache`（**实测成本价**，有效口径）；`measured_cost_price = cost ÷ official`（定价口径，M5 用）。分母为 0 时 null。
- `margin = (revenue − cost) ÷ revenue`，revenue = 0 时 null。

### 直方图（`stats.Hist`）

- 对数分桶，底数 `2^(1/8)`：`idx = floor(ln(v) / ln(base))`，v ≤ 0 丢弃。分位数返回桶的几何中点 `base^(idx+0.5)`，相对误差约 4.5%。
- 存储为 JSON 对象，键是桶号字符串：`{"52":3,"53":5}`。合并就是逐桶相加。首字延迟用毫秒，吐字速度用 token/秒，同一套分桶。

### 标准负载成本价与评级

- **厂商 token 构成**：该厂商最近 7 天 `stats_hourly` 的 `input、read、write(按 5m 计)、output` 之和。该厂商没有数据时用 `pricing.StandardRequest`（800 / 200 / 0 / 500）。
- **每个渠道**：模型按「单次成本」的规则选（测试模型有价就用它，否则 `CheapestModel`）。`std_cost_price = ChannelCNY(OfficialUSD(p, 构成, cr), disc, rate) ÷ OfficialUSD(p, 构成, {})`。缺价或未纳管为 null。
- **价格档**：同厂商、`std_cost_price` 非空的渠道 n ≥ 2 时，按升序排名 i（从 0 开始），`floor(3i/n)` 为 0/1/2 → `cheap`/`mid`/`expensive`；n < 2 为 null。
- **质量档**：requests < 20 → `insufficient`；`wilson ≥ 0.99` → `good`，`≥ 0.95` → `mid`，否则 `bad`。
- **错误日志提示** `error_log_hint`：最近 7 天成功数 > 0，且 KV `stats_error_log_seen_at` 为空或早于 7 天前。

### 厂商与上游归属

- `stats_hourly.vendor` 入库时确定：先找 `GroupName == log.group` 的厂商；找不到用 `channel_ext.vendor`；都没有为空串。
- 上游在查询时由 `channel_ext.upstream_id` 决定（渠道换上游后，历史数据跟着新上游走，接受）。

## 后端架构

### 表与设置（`internal/store`）

- `StatsHourly` 字段（表从未写过数据，可以直接改结构；`AutoMigrate` 只会加列）：
    - 主键 `ChannelID, Model, Hour`（Hour = `created_at` 截到整点的 Unix 秒）
    - `Group`、`Vendor`（新增，size 64，带索引）
    - `Success, Fail, FailByClass`（JSON，键见错误类别）
    - `PromptTokens`（**改为非缓存输入**，注释写清）、`CompletionTokens`、`CacheReadTokens`、`CacheWriteTokens`
    - `FRTHist`、`SpeedHist`、`SpeedEstHist`（新增）
    - `CostCNY, OfficialUSD, OfficialNoCacheUSD`（新增）、`PricedRequests`（新增）、`Quota, RevenueCNY`
- `LogCursor` 只用一行：`LogType = 0` 表示消费 + 错误合并游标。
- 新 KV 键和默认值：
    - `stats_backfill_days`（默认 7，范围 1–90）
    - `newapi_log_cleanup`（默认 `false`）
    - `stats_patch_p1_seen`、`stats_error_log_seen_at`、`stats_rebuild_since`（无默认值，内部用）

### 包 `internal/stats`（每个文件 ≤ 250 行）

- `sample.go`：`type Sample`（ChannelID、Model、Group、CreatedAt、Hour、OK、FailClass、Input、Read、W5、W1、Output、FRT、Speed、SpeedEst、Quota）和 `Parse(ports.LogRecord) (Sample, bool)`。纯函数。
- `hist.go`：`type Hist map[int]int64`，`Add/Merge/Count/Quantile/MarshalText/Parse`。
- `agg.go`：`type Agg`（与 StatsHourly 的统计字段一一对应，直方图用 Hist），`Add(Sample, Priced)`、`Merge(Agg)`、`FromRow/ApplyTo(row)`、`Metrics() Metrics`；`Wilson(s, n)`。
- `pricer.go`：`type Pricer`，`NewPricer(ctx, st, backend)`；`Price(Sample) Priced`（cost、official、officialNoCache、ok、revenue）；`VendorOf(group, channelID) string`。
- `ingest.go`：`Service.Pull(ctx) (string, error)`，即 `pull_logs` 任务体。
- `cleanup.go`：`Service.Cleanup(ctx) (string, error)`，即 `cleanup` 任务体。
- `rebuild.go`：`Service.Rebuild(ctx, days)`、`Service.Status(ctx)`。
- `latest.go`：「最新」档，见下。
- `query.go`：`ChannelStats`、`ChannelModels`、`Overview`、`Series`。
- `rating.go`：厂商构成、`std_cost_price`、价格档、质量档。

`Service` 结构：

```go
type Service struct {
	St       *store.Store
	Logs     ports.LogSource
	Channels ports.ChannelStore // 审计包装后的，只读用
	Backend  ports.Backend
	Sched    *scheduler.Scheduler // Pull 没追平时重新排队；Rebuild 触发拉取
	mu       sync.Mutex           // Pull / Cleanup / Rebuild 互斥
	latest   latestCache          // 60 秒缓存 + singleflight
}
```

### 增量拉取 `Pull`

- 持 `mu`。读游标：没有游标时 `since = KV stats_rebuild_since`（有就用并清掉）否则 `now − stats_backfill_days × 86400`；有游标时 `since = cursor.LastAt − 600`。
- 循环：`Logs.FetchAfter({AfterID: cursor.LastID, Since: since, Types: [2,5], Limit: 500})`。
    - 每批在**一个 nam.db 事务**里：内存里按主键合并成 `map[key]*Agg` → 逐键读出已有行、`Merge`、`Save` → 更新游标（`LastID` = 本批最大 id，含被跳过的测试日志；`LastAt` = 该条的 `created_at`）。聚合和游标一起提交，所以重复执行是幂等的。
    - 返回少于 500 条就结束。单次最多 20 批或 60 秒；没追平时 `Sched.Trigger(JobPullLogs, false)` 让它接着跑，消息写「已处理 N 条，未追平」。
- 返回消息：`处理 N 条（成功 a、失败 b、跳过 c），游标 #id`。

### 独立模式 `FetchAfter` 修正（`standalone/logs.go`）

- 现在的实现从最新页往旧页翻，回填 7 天时每次都要翻完整个范围。改为**从最旧的一页往新翻**：先请求第 1 页拿 `total`，`last = ceil(total/100)`，从 `last` 页往前读，收集 `id > AfterID` 的行，够 `Limit` 就停。新日志插入只会让旧行往后移，倒着读只会重复不会漏，按 id 去重。
- 多个类型时：每个类型各自收集；若某类型收满了 `Limit`，记它收集到的最大 id；`cutoff` 取这些最大 id 的最小值，合并后只保留 `id ≤ cutoff` 的行（都没收满就不截）。这样游标不会跳过还没读到的行。
- 加单测：两个类型、超过 500 条、交错 id，连续调用直到读完，结果与全集一致、无重复。

### 清理 `Cleanup`（`cleanup` 任务，默认每天）

- 聚合保留：`stats_retention_days > 0` 时删除 `hour < now − 天数 × 86400` 的行；0 表示不删。
- NewAPI 原始日志：只有 `newapi_log_cleanup == true`、有游标时才执行。`before = min(now − log_retention_days × 86400, cursor.LastAt)`，调 `Logs.DeleteBefore(before)`。<u>`before` 不超过游标时间，所以不会删到没拉取的日志</u>。
- 消息：`删除聚合 N 行；NewAPI 日志 M 条（或「未开启」）`。

### 重新统计 `Rebuild(days)`

- 持 `mu`：一个事务里删光 `stats_hourly` 和游标，写 `stats_rebuild_since = now − days × 86400`；释放锁后 `Sched.Trigger(JobPullLogs, false)`，清空 latest 缓存。days 范围 1–90。

### 「最新」档（`latest.go`）

- 候选渠道：NewAPI 里全部渠道中，`stats_hourly` 最近 24 小时有记录的（表为空时为空集，此时界面显示「暂无数据」）。
- 每个候选：`Recent(ch, 2, now−86400, 100)` 和 `Recent(ch, 5, now−86400, 100)`，合并后去掉测试日志，按 id 倒序取前 100，用 `Parse` + 当时构建的 `Pricer` 聚合成渠道级 `Agg` 和模型级 `map[model]Agg`。
- 串行执行，整体超时 20 秒，超时的渠道结果为空。结果缓存 60 秒，并发请求只算一次（`sync.Mutex` + 结果时间戳即可，不引入新依赖）。

### 查询（`query.go`）

- `ChannelStats(ctx, window)`：window 为 `latest`/`3d`/`7d`。3d/7d 读 `hour ≥ 当前整点 − 72h/168h` 的行按渠道合并。附上 `std_cost_price` 和两个评级（渠道列表、测试模型来自 `Channels.List`，折扣等来自 `channel_ext`）。
- `ChannelModels(ctx, id, window)`：同上，按模型合并。
- `Overview(ctx, from, to, dim)`：dim 为 `channel`/`model`/`upstream`/`vendor`。返回总计、排行行、错误类别合计、散点（渠道级 std_cost_price + wilson + requests）。
- `Series(ctx, from, to, dim, key, bucket)`：bucket 为 `hour`/`day`（day 按 `time.Local` 的自然日）。dim 为空表示全站。
- 时间参数是 Unix 秒，`from < to` 且跨度 ≤ 186 天，否则 400。

### 接口（`internal/api/stats.go`，契约同步写进 `docs/api/openapi.yaml`）

- `GET /stats/channels?window=latest|3d|7d`
- `GET /stats/channels/:id/models?window=`
- `GET /stats/overview?from&to&dim`
- `GET /stats/series?from&to&dim&key&bucket`
- `GET /stats/status`
- `POST /stats/rebuild {days}`
- 设置：`settingKeys` 加 `stats_backfill_days`、`newapi_log_cleanup`，`validateSetting` 加校验。
- `GET /backend`：`patch_p1 = info.PatchP1 || KV stats_patch_p1_seen`。
- `core`：`pull_logs` 和 `cleanup` 换成真实任务体；`api.Deps` 加 `Stats *stats.Service`。

### JSON 契约（前后端都按这个写）

`Metrics`（所有比率是 0–1 的小数，可空字段为 `null`）：

```json
{
  "requests": 120, "success": 118, "fail": 2,
  "success_rate": 0.983, "wilson": 0.94,
  "fail_by_class": {"5xx": 2},
  "frt_p50": 320, "frt_p95": 800,
  "speed_p50": 48.2, "speed_p95": 70.1, "speed_estimated": false,
  "cache_hit": 0.2,
  "prompt_tokens": 80000, "completion_tokens": 50000, "cache_read_tokens": 20000, "cache_write_tokens": 0,
  "cost_cny": 0.12, "revenue_cny": 0.3, "priced_requests": 118,
  "effective_cost_price": 0.166, "measured_cost_price": 0.175, "margin": 0.6
}
```

- `GET /stats/channels` → `{window, generated_at, patch_p1, error_log_hint, items: [Metrics + {channel_id, std_cost_price, price_tier, quality_tier}]}`  
  `price_tier`：`cheap|mid|expensive|null`；`quality_tier`：`good|mid|bad|insufficient`。
- `GET /stats/channels/:id/models` → `{items: [Metrics + {model}]}`
- `GET /stats/overview` → `{from, to, dim, patch_p1, error_log_hint, totals: Metrics, rows: [Metrics + {key, name}], error_classes: {"429": n, ...}, scatter: [{channel_id, name, vendor, std_cost_price, wilson, requests}]}`  
  `key`：渠道是 id 字符串；模型是模型名；上游是 id 字符串，未纳管为 `""`、name 为「未纳管」；厂商是厂商名，空为 `""`、name 为「未归属」。已删除的渠道 name 为 `#12（已删除）`。
- `GET /stats/series` → `{bucket, points: [Metrics + {t}]}`，只返回有数据的点，按 t 升序。
- `GET /stats/status` → `{cursor_id, cursor_at, rows, oldest_hour, backfill_days, patch_p1, error_log_hint, last_pull_at, last_pull_message}`
- `POST /stats/rebuild {days}` → `null`

## 前端

### 公共（主 agent 先写好）

- `web/src/api/types-stats.ts`：上面的类型。`web/src/api/endpoints.ts`：`statsChannels`、`statsChannelModels`、`statsOverview`、`statsSeries`、`statsStatus`、`statsRebuild`。

### 渠道页（`web/src/pages/channels/*`、`Settings.tsx`）

- 工具栏加「统计时长」下拉：最新 / 3 天 / 7 天，默认最新。按所选时长请求 `/stats/channels`，`refetchInterval` 60 秒。
- 列（替换掉现有的 `statColumn` 占位）：
    - 默认显示：成功率（百分比；悬停显示请求数、失败数、Wilson 下界、错误类别）、首字延迟（p50 毫秒，悬停 p95）、吐字速度（p50，估算时加灰色「估」字并在悬停里说明「NewAPI 未打补丁 P1，按整秒估算」）、缓存命中率、**实测成本**（`effective_cost_price`，颜色梯度，悬停说明「每消耗原厂 1 美元（不算缓存）实际花多少元，缓存命中越高越低」）
    - 默认隐藏：定价口径成本价、标准负载成本价（颜色梯度）、价格档、质量档、成本（元）
    - 现有的「上游渠道成本价」「单次成本」「手动缓存率」保留
- 组标题行：组名、渠道数、平均成功率（按请求数加权）、成本合计。
- `error_log_hint` 为真时，在表格上方显示黄色提示条：「最近 7 天没有任何错误日志，NewAPI 可能没开 `ERROR_LOG_ENABLED`，成功率会偏高」。
- 下表模型明细：请求数、成功率、首字延迟、吐字速度四列用 `/stats/channels/:id/models` 填。
- 设置页「日志保留」分区：
    - 「清理 NewAPI 原始日志」开关（默认关），打开前确认框说明：会删除 NewAPI 里早于保留天数的**全部类型**日志（含充值、管理日志），只删已统计过的部分。
    - NewAPI 原始日志保留天数、聚合保留天数、首次回填天数。
    - 统计状态：游标、已统计到的时间、聚合行数、最早数据、补丁 P1 是否检测到。
    - 「清空并重新统计」按钮：弹窗选天数（1–90，默认取回填天数），确认后调 `/stats/rebuild`。

### 统计页（`web/src/pages/Stats.tsx`、`web/src/pages/stats/*`）

- 用 `React.lazy` 加载，recharts 只进这个分块。`package.json` 加 `"recharts": "2.15.4"`（固定版本）。
- 时间范围：24 小时、7 天、30 天、自定义（两个原生 `<input type="date">`）。维度：渠道、模型、上游、厂商。
- 图表：
    - 请求量（柱）+ 成功率（线，右轴）
    - 首字延迟 p50/p95 + 吐字速度 p50/p95（两个小图）
    - 成本 vs 收入（柱）
    - 价格-质量散点：横轴标准负载成本价，纵轴 Wilson 下界，点大小 = 请求数，悬停显示渠道名和厂商
    - 错误类别分布（横向柱）
- 排行榜：名称、请求数、成功率、失败数、首字延迟 p50/p95、吐字速度、成本、收入、毛利率；点击表头排序；「导出 CSV」（带 BOM，Excel 打开不乱码）。
- 渠道维度点击行打开右侧抽屉：关键指标、逐小时图（series bucket=hour）、逐小时表（成功、失败、失败类别、首字延迟、吐字速度、缓存率、成本），「在渠道页打开」跳 `/channels?search=<渠道名>`（渠道页读这个参数填入搜索框）。
- `error_log_hint`、未打补丁（吐字速度为估算）在页头显示提示。本页只读。

## NewAPI 补丁 P1

- 仓库 `temp/clansty-new-api`，分支 `nam-patches`，单独一个提交 `feat: log use_time_ms`。
- `service/log_info_generate.go` 的 `GenerateTextOtherInfo` 里加 `other["use_time_ms"] = time.Since(relayInfo.StartTime).Milliseconds()`（Claude、音频、WSS 都经过这个函数）。不改 `frt`。
- 测试：`service` 包内新测试，构造 `StartTime = now − 2.5s` 的 relayInfo，断言 `use_time_ms` 在 [2500, 3000)。遵守该仓库 `AGENTS.md`（JSON 用 `common.*`）。
- `go test ./service/... ./model/...` 通过。

## 任务划分

### 依赖关系

- T0（主 agent）→ T3、T4 可以开始
- T1 → T2 → T6
- T5 独立，T6 依赖它
- T3、T4 与 T1、T2 并行，只依赖 JSON 契约
- 全部完成后 T7 审查

### 任务清单

- **T0 前端类型与接口函数**（主 agent 自己写）
- **T1 后端核心**（opus，`unspecified-high`，技能 `programming`）
    - store 改动；`internal/stats` 的 sample、hist、agg、pricer、ingest、cleanup、rebuild；独立模式 `FetchAfter` 修正；`core` 里 `pull_logs`、`cleanup` 换成真实任务体
    - 单测：`Parse` 全部分支；`Hist` 分位数；`Metrics`（用下面的数值）；`Pull` 幂等（同一批日志跑两次总数不变、跨 500 条批次边界、测试日志被跳过）；`Cleanup` 不越过游标、开关关闭不删；独立模式翻页
- **T2 后端查询与接口**（sonnet，`unspecified-low`，技能 `programming`）
    - latest、query、rating；`api/stats.go`；设置键；`/backend` 的 patch_p1；`openapi.yaml`；接口级测试（用 `portstest.Fake` 加日志）
- **T3 渠道页与设置页**（sonnet，`visual-engineering`，技能 `frontend`）
- **T4 统计页**（sonnet，`visual-engineering`，技能 `frontend`）
- **T5 补丁 P1**（sonnet，`unspecified-low`）
- **T6 端到端**（sonnet，`unspecified-low`）
    - 重新编译带 P1 和 nambridge 的 `new-api-nam.exe`；`temp/e2e/run.ps1` 启动时加 `ERROR_LOG_ENABLED=true`；新脚本 `temp/e2e/m4.ps1` 按下面 QA 场景断言；M2/M3/M3.5/M3.6 回归
- **T7 审查**（oracle）：对照本计划审查 T1–T6 的 diff，主 agent 再逐项复核

### 文件归属（避免并行任务改同一文件）

- T1：`nam/internal/store/*`、`nam/internal/stats/{sample,hist,agg,pricer,ingest,cleanup,rebuild}.go`、`nam/standalone/logs.go`、`nam/core/core.go`
- T2：`nam/internal/stats/{latest,query,rating}.go`、`nam/internal/api/*`、`nam/core/core.go`（只加 Deps 一行）、`docs/api/openapi.yaml`
- T3：`web/src/pages/channels/*`、`web/src/pages/Channels.tsx`、`web/src/pages/Settings.tsx`、`web/src/pages/settings/*`
- T4：`web/src/pages/Stats.tsx`、`web/src/pages/stats/*`、`web/src/app/routes.tsx`、`web/package.json`、`web/package-lock.json`
- T5：`temp/clansty-new-api/service/*`
- T6：`temp/e2e/*`、`nam/cmd/mockupstream/*`（如需要按 key 控制错误）

## QA

### 单测里的固定数值

- 标准请求（prompt 1000、cache 200、completion 500，原厂价 输入 3 / 输出 15 / 缓存读 0.3），折扣 0.35，汇率 0.5：
    - `official = 0.00996`，`officialNoCache = 0.0105`，`cost = 0.001743`
    - `measured_cost_price = 0.175`，`effective_cost_price ≈ 0.166`，`cache_hit = 0.2`
    - 没有缓存覆盖时 `std_cost_price = 0.175`（= 折扣 × 汇率）
- Wilson：10 次全成功 ≈ 0.722；1000 次 990 成功 ≈ 0.9817。

### 端到端（`temp/e2e/m4.ps1`）

- 前置：两个渠道 a、b 指向 mock；让渠道 a 的前 2 次请求返回 500（mock 若只能全局控制，就把 b 暂时禁用）。
- 发 20 次流式 + 10 次非流式请求，再对渠道 a 执行一次 NewAPI「测试」，然后 `POST /api/nam/jobs/pull_logs?wait=1`。断言：
    - 所有渠道成功数之和 = 30（测试日志被排除）
    - 渠道 a 失败 = 2，类别 `5xx`
    - 首字延迟 p50 在 [250, 450] 毫秒；吐字速度 p50 在 [200, 300] token/秒，`speed_estimated=false`
    - 缓存命中率 = 0.2（误差 ≤ 0.001）
    - 「最新」档渠道 a 的请求数 ≤ 100，且等于 3 天档的请求数
    - 再跑一次 `pull_logs`，总数不变
- 对账：nam 的收入合计与 `Σquota ÷ 500000 × 7` 相对误差 ≤ 1e-6；成本合计与逐条日志按本计划公式复算的结果相对误差 ≤ 1e-6。
- 补丁：新日志的 `use_time_ms` 在 [2200, 2700]；`/backend` 的 `patch_p1=true`。
- **重新统计**（必须在清理之前）：`POST /api/nam/stats/rebuild {days:1}`，再 `pull_logs?wait=1`。成功数之和仍 = 30，渠道 a 失败仍 = 2。
- **清理**（最后一步）：`PUT /settings {newapi_log_cleanup:"true", log_retention_days:"0"}`，再发 1 次请求（不拉取），然后 `POST /jobs/cleanup?wait=1`。断言：
    - NewAPI `GET /api/log/?type=2` 里只剩这 1 条未拉取的日志（`created_at` ≥ 游标时间的行可能多保留几条，断言「未拉取的那条还在」且「最早的测试前日志已不在」）
    - `stats_hourly` 的成功数之和仍 = 30
    - 关闭开关后再跑一次 `cleanup`，NewAPI 日志条数不变

### T2 接口级测试（`internal/api/stats_test.go` 或 `core/m4_test.go`）

- 夹具：`portstest.Fake` + `core.New`，建上游（汇率 0.5）、厂商 anthropic（分组 `anthropic`）、渠道 1 和 2（都纳管，折扣 0.35，测试模型 `test-model`），原厂价 `test-model` 输入 3 / 输出 15 / 缓存读 0.3。`AddLog` 写入：
    - 渠道 1：10 条消费日志，`prompt 1000、completion 500、is_stream=true、quota=1743、group=anthropic`，`other={"cache_tokens":200,"frt":300,"use_time_ms":2300}`，`created_at` = 现在 − 1 小时
    - 渠道 1：2 条错误日志，`other={"status_code":500}`
    - 渠道 1：1 条 `token_name="模型测试"` 的消费日志
    - 渠道 2：5 条消费日志（同上，但 `other` 没有 `use_time_ms`，`use_time=2`）
    - 先 `POST /jobs/pull_logs?wait=1`
- 断言：
    - `GET /stats/channels?window=3d`：渠道 1 `success=10, fail=2, fail_by_class={"5xx":2}`，`cache_hit=0.2`，`effective_cost_price≈0.166`（1e-6 相对误差，按 `0.001743/0.0105`），`measured_cost_price=0.175`，`speed_estimated=false`，`speed_p50` 在 250 ± 5%（500 ÷ 2.0s）；渠道 2 `speed_estimated=true`，`speed_p50` 在 [290, 310]（500 ÷ 1.7s ≈ 294）；两者 `std_cost_price=0.175`；`price_tier` 都不为 null；渠道 1 `quality_tier="insufficient"`（12 < 20）
    - `window=latest`：渠道 1 的 `success`、`fail` 与 3d 相同
    - `GET /stats/channels/1/models?window=7d`：一行 `model=test-model`，`success=10`
    - `GET /stats/overview?dim=vendor&from=..&to=..`：一行 `key=anthropic`，`success=15`；`totals.revenue_cny = 15 × 1743 ÷ 500000 × 7`；`error_classes={"5xx":2}`；`scatter` 两个点
    - `GET /stats/series?bucket=hour&from=..&to=..`：恰好 1 个点，`success=15`
    - `error_log_hint=false`（有错误日志）；删掉错误日志、`rebuild` 再拉取后 `error_log_hint=true`
    - `GET /stats/status`：`cursor_id` = 最大日志 id，`rows ≥ 1`
    - 参数错误：`window=x`、`from>to`、跨度 > 186 天、`rebuild days=0/91` → 400
    - `PUT /settings {stats_backfill_days:"0"}` → 400；`{newapi_log_cleanup:"yes"}` → 400
    - `GET /backend`：拉取后 `patch_p1=true`

### 前端

- `npm run build` 通过；`dist/assets` 里首页入口 js 不含字符串 `recharts`，另有一个统计页分块含它。
- 浏览器手测（主人执行，开发环境 `npm run dev` + 独立模式，或嵌入二进制）：
    - **渠道页**
        - 打开渠道页：统计时长默认「最新」；有流量的渠道成功率、首字延迟、吐字速度、缓存命中率、实测成本有值，没流量的显示「—」
        - 切到「3 天」「7 天」：数值刷新，不报错
        - 悬停成功率：显示请求数、失败数、Wilson 下界和错误类别；悬停实测成本：显示说明文字
        - NewAPI 没打补丁时，吐字速度旁有灰色「估」字
        - 「列」里能打开定价口径成本价、标准负载成本价、价格档、质量档、成本（元）；请求数 < 20 的渠道质量档显示「样本不足」
        - 组标题行显示平均成功率和成本合计
        - 点一个渠道，下表模型明细的请求数、成功率、首字延迟、吐字速度有值
        - 没开错误日志时，表格上方出现黄色提示条
    - **统计页**
        - 首次进入时，网络面板里多加载了一个统计页的 js 分块；切换 24 小时 / 7 天 / 30 天 / 自定义，图表和排行榜都会刷新
        - 切换维度渠道 / 模型 / 上游 / 厂商，排行榜第一列跟着变
        - 点表头能升降序；「导出 CSV」用 Excel 打开中文不乱码，列与表格一致
        - 散点图悬停显示渠道名和厂商；错误类别图与排行榜失败数之和一致
        - 渠道维度点击一行打开抽屉，有逐小时图和表；「在渠道页打开」跳到渠道页并已填好搜索
    - **设置页**
        - 「清理 NewAPI 原始日志」开关默认关；打开时弹确认框，取消则开关保持关闭
        - 统计状态显示游标、已统计到的时间和聚合行数
        - 「清空并重新统计」选 1 天确认后，几十秒内状态里的游标重新增长，渠道页数字恢复
