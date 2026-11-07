# NewAPI 管理工具与补丁计划（v0.1 草案）

> **第 18 轮修订**：参数改用中文名（原厂价、上游分组折扣、上游货币汇率、上游渠道成本价、售价、站点汇率、加价比例）；定价改为"厂商分组 + auto，nam 只维护厂商分组倍率"；UI 从零重新实现；补全 M0 / 数据模型 / 工程细节的缺口。
>
> 优先级算法和故障转移策略要另外讨论几轮（见 §9）。本计划只为它们留出接口，不定具体策略。

## 0. 目标与范围
- 目标：用一个管理工具（默认嵌入 NewAPI 运行，也可独立运行）集中管理 10–30 个上游的渠道、价格、余额和统计，并**自动维护各厂商分组的倍率**（不改每个模型自己的价格）。
- 范围内：上游和渠道配置、批量编辑、上游分组折扣与余额同步、渠道统计、按厂商设售价并自动写入厂商分组倍率、通知、NewAPI 镜像的构建流程。
- 范围外：
    - 朋友的权限和额度：在 NewAPI 里手动管理。
    - 用户分组加价（朋友贵一点）：**以后**再做，现在所有用户同一个倍率。
    - 模式 1（按实际渠道成本计费）：只保留所需数据，以后再做。
    - 优先级算法和故障转移的具体策略：§9 讨论后另写计划。

## 1. 已确认的事实与决定
| 项目 | 内容 |
|---|---|
| NewAPI | clansty 分支（HEAD d292f5e8，底座 v0.13.2）；Docker 单实例，SQLite，服务器 1c2g |
| 构建 | 本地 WSL 构建 linux/amd64 镜像后上传 |
| 工具 | Go + React（TypeScript）。默认嵌入 NewAPI 进程运行，也能独立运行；只允许 root 使用；数据存 `nam.db` |
| 上游类型 | NewAPI 系、Sub2API、OneAPI/OneHub/DoneHub、官方直连、别人定制的中转软件 |
| 上游凭证 | Access Token、账号密码、只有 sk-key；账号密码允许加密保存 |
| 渠道成本 | 成本(元) = 原厂价(美元) × 上游分组折扣 × 上游货币汇率。**上游分组折扣** = 上游价格 ÷ 原厂价（0.3 = 三折）；**上游货币汇率** = 在这个上游买 1 美元额度要花多少元，每个上游一个固定值；两者相乘叫**上游渠道成本价**（元 / 原厂 1 美元用量，例如 0.3 × 2 = 0.6 元） |
| 缓存价 | 默认按原厂比例，个别渠道手动覆盖 |
| 原厂价 | 自动同步公开价格库，个别模型手动覆盖；单位美元每百万 token，或美元每次 |
| 售价 | 按厂商设**售价**（元 / 原厂 1 美元用量，直接填人民币；固定，或跟随参考成本价 × 加价比例）：用户支付（元）= 原厂价（美元）× 售价；所有用户同一个价；自动写入厂商分组倍率，变化超过阈值时待确认（见 §8） |
| 站点汇率 | 现在 1 美元额度卖 7 元，以后可能改为 1:1，必须可调 |
| 分组 | **每个厂商一个 NewAPI 分组**（如 `claude`、`openai`、`gemini`），该厂商所有渠道的 `group` 只填这一个组；用户令牌用 `auto`；nam 自动维护厂商分组的倍率和 `AutoGroups`；不再使用 `self,friend` |
| 开发环境 | Windows 写代码、跑单元测试：Go 1.25.x 装在 `D:\Go`（GOPATH、GOMODCACHE、GOCACHE 都在该目录下），Node 用现有的 v24。WSL 只用 docker（29.1.3）运行 NewAPI、做 M0/QA、构建镜像；镜像内用自己的 Go 编译 |
| 请求量 | 每天 1000–1 万次 |
| 渠道测试 | 只测被禁用的渠道 |
| 通知 | Telegram、Webhook、邮件；上游公告和价格变动也要提醒 |

## 2. 总体架构（第 13–16 轮修订，详见 `docs/requirements.md` §4.9、`docs/feature-split.md`）
```
嵌入模式（生产默认）:
浏览器 ──HTTPS──> 反向代理 ──> new-api 进程
                                ├─ NewAPI 原有功能（clansty + 少量补丁）
                                ├─ nambridge（嵌入适配器，位于 new-api 仓库）
                                └─ nam 核心（Go 模块）+ /nam/ 前端（embed）── nam.db
独立模式（开发 / 退路）:
浏览器 ──> nam 进程（核心 + 独立适配器 + 自带登录）── nam.db
            └──HTTP──> NewAPI 管理 API（root 访问令牌 + New-Api-User 头）
两种模式下 nam ──公网──> 各上游站点、公开价格库、Telegram / Webhook / 邮件
```
- 工具代号 `nam`（newapi-manager），名称可改。
- 原则 1：nam 核心不引用 new-api 的包，只通过 `ChannelStore`、`OptionStore`、`LogSource`、`Auth` 四个接口访问 NewAPI。CI 中单独编译核心模块，检查这一点。
- 原则 2：写 NewAPI 必须走 NewAPI 自己的完整流程。嵌入适配器复用 controller 的渠道增改逻辑（校验、abilities 重建），之后调 `InitChannelCache()`；写配置项走 `model.UpdateOption`，之后调 `InvalidatePricingCache()`。不直接用 SQL 改 NewAPI 的表。
- 原则 3：渠道以 NewAPI 为准。nam 只按 `channel_id` 保存扩展信息，不复制渠道字段；漂移检测只检查 nam 管理的字段（模型、优先级、权重、标签、状态、测试模型、base_url）。
- 原则 4：所有写入都记入审计日志，并保存写入前的值，以便回滚。
- 原则 5：只允许 root（角色 100）使用。嵌入模式所有接口套 `RootAuth()`，高危操作再加 `SecureVerificationRequired()`。
- 原则 6（嵌入模式硬性要求）：所有后台 goroutine 都经过统一的 `safeGo` 包装（recover 后记日志并告警）；跟随 `common.ShutdownCtx()` 退出；nam 数据只写 `nam.db`，写 NewAPI 时用小事务。
- 资源预算：nam 新增内存不超过 100MB；同步任务串行执行；独立模式下访问 NewAPI 的请求间隔不小于 200ms。

## 3. NewAPI 端：配置变更（不改代码）
| 项目 | 做法 | 原因 |
|---|---|---|
| 错误日志 | docker-compose 中加环境变量 `ERROR_LOG_ENABLED=true` | 默认关闭；不开启就无法按渠道统计失败次数 |
| 渠道类型 | 渠道一律用 type=58 PassThrough，base_url 填上游站点根地址（不带 /v1） | 原样转发 OpenAI、Responses、Claude、Gemini 四种格式，一把 key 走三家；type=1 会把 Claude/Gemini 请求转换成 chat/completions |
| 自动禁用 | 开启 `AutomaticDisableChannelEnabled`，同时开启 `AutomaticEnableChannelEnabled` | 内置的错误禁用和定时恢复已经满足"只测被禁用的渠道" |
| 定时测试 | `monitor_setting.auto_test_channel_enabled=false`，`auto_test_channel_minutes` 按需设置 | 关闭全量测试后，调度循环只测自动禁用的渠道（channel-test.go:1126-1136）；需在 M0 实测确认 |
| 测试模型 | 工具为每个渠道写 `test_model`（选该渠道最便宜的模型） | 测试同样要走价格计算，模型没有定价时测试会直接失败 |
| 分组 | nam 写 `GroupRatio`（每个厂商分组一项）、`AutoGroups`（厂商分组的顺序）、`UserUsableGroups`（加入厂商分组）；渠道的 `group` 只填所属厂商分组；用户令牌用 `auto`。nam **不**开启 NewAPI 的"默认使用 auto 分组" | auto 令牌会选 `AutoGroups` 中第一个对该模型有可用渠道的组，并按 `GroupRatio[选中的组]` 计费。**已知限制**：不用 auto 的令牌会绕过这个机制，需要用户自己选 auto |
| 模型价格 | `ModelRatio`、`CompletionRatio`、`CacheRatio`、`CreateCacheRatio`、`ModelPrice` 保持原厂价，由 NewAPI 自带的价格同步维护，nam 不自动写入 | 不想随意改动每个模型自己的价格；nam 只做偏差检查（见 §8） |
| 站点汇率 | `Price=7`、`USDExchangeRate=7`、`general_setting.quota_display_type` 按需设置；`QuotaPerUnit` 保持 500000 不变 | `Price` 是"1 美元额度卖多少元" |

## 4. NewAPI 端：代码补丁（尽量少）
| 编号 | 内容 | 必要性 | 改动位置（预估） |
|---|---|---|---|
| P1 | 消费日志的 `other` 增加 `use_time_ms`（毫秒总耗时）；非流式请求的 `frt` 改为不写或写 null，不再固定写 -1000 | 必需。`use_time` 只有整秒，算出的吐字速度误差太大 | `service/log_info_generate.go`、`model/log_record.go`，约 20 行 |
| P2 | 让 `CompletionRatio` / `CacheRatio` 配置表的值优先于代码里硬编码的倍率 | **大概率不需要**。nam 不再写模型价格，只在 M0 ④ 核对 NewAPI 实际生效的模型价格是否等于原厂价，用来校准偏差警告 | `setting/ratio_setting/model_ratio.go`，约 10 行 |
| P3 | 为故障转移预留，例如按模型设置优先级、熔断 | 待 §9 讨论 | 未定 |
| P4 | `GET /api/log/` 增加 `min_id` 参数，并在带 `min_id` 时跳过 COUNT 查询 | 可选，仅独立模式需要。嵌入模式直接查库，不需要 | `controller/log.go`、`model/log.go`，约 20 行 |

补丁的维护方式：
- 在 clansty 分支上建自己的分支 `nam-patches`，每个补丁一个提交，并附单元测试。
- clansty 更新后，把补丁 rebase 到新版本上。
- 构建流程：在 WSL 中执行 `docker build --platform linux/amd64 -t new-api:nam-<版本> .`，然后执行 `docker save | gzip`，上传到服务器后 `docker load`，修改 compose 中的镜像标签，再 `up -d`。
- 保留上一版镜像，回滚时只需改回标签。

## 5. 工具数据模型（nam 自己的 SQLite）
| 表 | 主要字段 | 说明 |
|---|---|---|
| `upstream` | 名称、类型（newapi / sub2api / onehub / official / generic）、站点地址、凭证类型、加密凭证、平台用户 ID、上游货币汇率（`currency_rate`）、标签、备注、适配器配置（JSON） | 一个上游站点 |
| `upstream_group` | 上游、分组名、上游给出的分组倍率、更新时间 | 从上游同步，用于推算上游分组折扣 |
| `upstream_token` | 上游、令牌名、所属分组、加密 key、额度和状态 | 从上游同步，也可以手动录入 |
| `upstream_model_price` | 上游、分组、模型、输入价、输出价、缓存读价、缓存写价、按次价、更新时间 | 适配器 `Pricing` 返回的各模型上游价格 |
| `channel_ext` | NewAPI 渠道 ID（主键）、上游、令牌、厂商、上游分组折扣（`discount`）、折扣来源（`discount_source`，自动 / 手动）、缓存倍率覆盖、模型级折扣覆盖、上次应用时 nam 管理字段的哈希 | 只保存扩展信息，渠道本身的字段每次从 NewAPI 实时读取 |
| `channel_draft` | 渠道 ID（新建时为临时 ID）、操作（新建 / 修改 / 删除）、字段差异 JSON | 待应用的草稿 |
| `vendor` | 厂商名、NewAPI 分组名、模型匹配规则或模型列表、在 `AutoGroups` 中的顺序 | 一个厂商一个分组；一个模型名只能属于一个厂商 |
| `vendor_pricing` | 厂商、模式（固定 / 跟随）、售价（`sell_price`）、加价比例、上次写入的售价、状态（正常 / 待确认）、待确认的新值 | 厂商的售价与保护状态 |
| `official_price` | 模型、厂商、输入价、输出价、缓存读价、5 分钟和 1 小时缓存写价、按次价格、来源、更新时间 | 原厂价库，单位为美元每百万 token（按次为美元每次） |
| `price_override` / `model_alias` | 手动覆盖的价格；上游模型名到原厂模型名的映射 | 用来处理价格库缺失和模型改名 |
| `stats_hourly` | 渠道、模型、小时；成功和失败次数（按错误类别细分）；各类 token 数；首字延迟和吐字速度的直方图；成本（元） | 只保存聚合结果，不保存原始日志 |
| `log_cursor` | 日志类型、已拉取到的最大 id 和时间 | 增量拉取的游标 |
| `price_history` / `audit_log` | 每次写入 NewAPI 的内容、写入前的值、触发原因 | 用于回滚 |
| `notify_channel` / `notify_event` | 通知通道配置；事件去重 | 同一事件在冷静期内只通知一次 |
| `snapshot` | 上游公告和价格的哈希 | 用来检测变化 |
| `setting` | 站点汇率（初始 7）、变化保护阈值（默认 50%）、自动写入开关、各项调度间隔 | 均可在界面修改 |

- 以上所有表都在 `nam.db`，两种运行模式相同；不放进 NewAPI 的数据库。
- 凭证使用 AES-GCM 加密，主密钥来自环境变量 `NAM_SECRET`；数据库备份不包含主密钥。
- `nam.db` 开启 WAL，每天把备份文件写到挂载卷。
- nam 核心的 SQLite 驱动与 NewAPI 保持一致（`glebarez/sqlite` + gorm），避免嵌入后出现两套驱动或版本冲突。

## 6. 上游适配器与同步
适配器接口：`Probe`（识别站点类型）、`Balance`、`ListTokens`、`CreateToken`（可选）、`Pricing`（返回分组倍率、模型倍率，以及各模型的上游价格，写入 `upstream_model_price`）、`Announcements`（可选）、`ListModels`（使用令牌 key）。

| 适配器 | 认证方式 | 余额 | 令牌和分组 | 倍率 | 公告 |
|---|---|---|---|---|---|
| newapi（含 VoAPI、Veloera 等） | Access Token + 用户 ID；也可用账号密码登录换取 session | `/api/user/self`，单位按对方 `/api/status` 的 `quota_per_unit` 换算 | `/api/token/`、`/api/user/self/groups` | `/api/pricing` | `/api/notice` |
| newapi，只有 sk-key | key | `/v1/dashboard/billing/subscription` 和 `/v1/dashboard/billing/usage` | 无 | 上游分组折扣手动填写 | 无 |
| sub2api | JWT；账号密码登录在 M0 验证 | `/api/v1/auth/me` | 待 M0 确认 | 待 M0 确认 | `/api/v1/announcements` |
| onehub / donehub | Access Token | `/api/user/self` | `/api/token/` | `/api/available_model`、`/api/user_group_map` | `/api/notice` |
| official（官方直连） | key | 无 | 无 | 上游分组折扣默认 1，可手动设折扣 | 无 |
| generic（定制中转软件） | 可配置请求头 | 可配置"请求地址 + JSON 路径 + 单位换算" | 无 | 上游分组折扣手动填写 | 可配置页面地址，按哈希检测变化 |

上游分组折扣的自动推算（newapi 类）：**上游分组折扣 = 上游模型倍率 × 上游分组倍率 ÷ 原厂模型倍率**。
- 每个渠道先算出各模型的上游分组折扣，再取中位数。
- 某个模型的折扣和中位数相差超过 5% 时，在界面标出，可以只对这个模型手动覆盖。

调度（串行执行，间隔可调）：
| 任务 | 默认间隔 |
|---|---|
| 余额同步 | 30 分钟 |
| 倍率同步 | 6 小时 |
| 公开价格库同步 | 每天一次（数据源为 LiteLLM 的 `model_prices_and_context_window.json` 和 models.dev，M0 选定主源） |
| 公告检查 | 6 小时 |
| 拉取 NewAPI 日志 | 2 分钟 |
| 定价计算并写入 | 30 分钟 |
| 渠道一致性检查 | 10 分钟 |

## 7. 渠道统计
数据来源：NewAPI 的日志，type=2 是消费日志，type=5 是错误日志。通过 `LogSource` 接口读取：
- 嵌入模式：直接查 `LOG_DB` 的 `logs` 表，`WHERE id > 游标 ORDER BY id LIMIT 500`，只选需要的列；不调用 `GetLogPerformance`、`SumLogStat`。
- 独立模式：用下面的 HTTP 拉取方式。

独立模式的拉取方式：
- 每页 100 条，按 id 倒序翻页，碰到已拉取过的 id 就停。
- 时间窗口往前多取 120 秒作为重叠，按 id 去重。
- 每次请求都必须带 `start_timestamp`。不带时间范围时，NewAPI 的 COUNT 查询会扫全表。
- 不调用 `/api/log/stat` 和 `/api/log/performance`。这两个接口会把整个时间范围内的日志全部读一遍，在 1c2g 上太重。

过滤规则：
- 去掉 `token_name == "模型测试"` 的记录，这些是渠道测试产生的日志。
- 消费日志中的 `channel` 字段是最终成功的渠道。
- 错误日志按每次失败的尝试各记一次，归到它自己的渠道上。用 `request_id` 把同一次请求的成功和失败日志关联起来。

各项指标：
| 指标 | 计算方法 |
|---|---|
| 成功率 | 成功次数 ÷ (成功次数 + 失败次数)，按渠道、按小时统计。展示时用 Wilson 下界，避免样本少时数字虚高 |
| 首字延迟 | `other.frt`（毫秒）。只统计 `is_stream=true` 且 frt ≥ 0 的请求 |
| 吐字速度 | completion_tokens ÷ (P1 补丁加的毫秒总耗时 − frt)，只统计流式请求 |
| 缓存命中率 | 用 `cache_tokens` 和 `cache_write_tokens`（或 `cache_creation_tokens_5m` 加 `_1h`）计算。Anthropic 语义下，分母要把缓存 token 也算进去 |
| 成本 | 不使用 NewAPI 的 `logs.cost` 字段，因为它依赖手动填的上游倍率。工具按 token 构成 × **原厂价 × 上游分组折扣 × 上游货币汇率** 自己重新计算，结果是人民币 |
| 收入 | quota ÷ QuotaPerUnit × 站点汇率，用来核对售价是否达到预期 |
| 厂商归属 | 用消费日志的 `group` 字段（auto 令牌选中的厂商分组）归到厂商 |
| 实测成本价 | 渠道自己的实际成本（元，已计入缓存）÷ 同一批请求的原厂成本（美元），用该渠道自己的流量 |
| 标准负载成本价 | 用一套固定的标准 token 构成（该厂商全站最近 7 天的 token 构成：输入、缓存读、缓存写、输出各占多少），按每个渠道自己的价格计价，再除以同一构成的原厂成本。不受渠道自己流量构成的影响，渠道之间可以直接比较 |
| 评级 | **价格档**：标准负载成本价在同厂商内排名，前 1/3 为便宜，中间为中，后 1/3 为贵。**质量档**：Wilson 下界成功率 ≥ 99% 为好，≥ 95% 为中，其余为差。散点图：横轴标准负载成本价，纵轴成功率 Wilson 下界，点大小为请求数。档位阈值以后可调 |

存储与展示：
- 统计分三档时间范围：
    - **最新**：每个渠道最近 24 小时内的最近 100 次请求，**直接查 NewAPI 的日志，不另存样本**。嵌入模式查 `LOG_DB`：`WHERE channel_id=? AND created_at>=now-24h ORDER BY id DESC LIMIT 100`（`channel_id` 有索引），只选需要的列，消费日志（type 2）和错误日志（type 5）各查一次，结果缓存约 1 分钟。独立模式用 `GET /api/log/` 按渠道过滤并带 `start_timestamp`（渠道过滤是否可用见 M0 ⑬）。
    - **3 天 / 7 天**：读 `stats_hourly`。
- 统计结果写入 `stats_hourly`，只保留聚合数据。首字延迟和吐字速度用对数直方图保存，可以算出 p50 和 p95。
- 聚合数据保留 180 天。
- NewAPI 自己的原始日志由工具定期清理（嵌入模式调用 `model.DeleteOldLog`，独立模式调用 `DELETE /api/log/`），默认保留 30 天，可以调整。

## 8. 定价：厂商分组 + auto，nam 只写分组倍率
依据 `docs/requirements.md` P1–P5（第 18 轮）。旧的"按模型把售价乘进每个 ModelRatio"和"按模型 7 天加权成本"方案已废弃。

核心思路：
- 每个厂商一个 NewAPI 分组，该厂商所有渠道的 `group` 只填这个分组；用户令牌用 `auto`。
- NewAPI 的模型价格保持原厂价，**nam 不自动写**。
- 每个厂商一个**售价**（元 / 原厂 1 美元用量，直接填人民币）。nam 写 `GroupRatio[厂商分组] = 售价 ÷ 站点汇率`，例如 1.5 ÷ 7 = 0.2143。
- 用户实际支付（元）= 原厂价（美元）× 售价。现在所有用户同一个价。
- NewAPI 对 auto 令牌的行为（已在 clansty 源码确认）：`service/channel_select.go` 选 `AutoGroups` 中第一个对该模型有可用渠道的组；`relay/helper/price.go` 的 `HandleGroupRatio` 按 `GroupRatio[选中的组]` 计费；日志有 `group` 字段。

每个厂商有两种模式：
- **固定**：售价由主人手填。
- **跟随参考成本**：售价 = 参考成本价 × 加价比例。
    - 参考成本价 = 最近 7 天该厂商所有渠道的实测成本价，按原厂成本加权平均。
    - 冷启动：该厂商 7 天内没有流量时，取 priority 最高的那一层渠道的上游渠道成本价的平均值。

写入 NewAPI 的内容（只有这几项）：
- `GroupRatio`（每个厂商分组一项）、`AutoGroups`（厂商分组顺序）、`UserUsableGroups`（加入厂商分组）。
- `Price` 和 `USDExchangeRate`（随站点汇率）。
- 计算由定时任务或"立即计算"触发；"自动写入"开关关闭时只计算不写入。
- 模型价格（`ModelRatio`、`CompletionRatio`、`CacheRatio`、`CreateCacheRatio`、`ModelPrice`）**不写**。

写入流程（通过 `OptionStore`，只用于上面几项）：
1. 先读出当前完整配置表。
2. 在本地合并修改，校验 JSON 格式。
3. 逐个 key 写入并检查结果：嵌入模式调用 `model.UpdateOption` 后 `InvalidatePricingCache()`；独立模式发 `PUT /api/option/`（value 必须是 JSON 字符串）并检查 `success`。
4. 读回来核对。

保护措施：
- 跟随模式下，售价相对上次写入值的变化超过"变化保护阈值"（默认 50%）时，该厂商进入"待确认"，不写入并发通知；主人确认后按新值写入。固定模式下的手动修改不受此限制，但写入前要在预览中确认。
- 价格源缺失时，这个模型不计入参考成本价，状态为"缺价"并告警，`GroupRatio` 不变。
- 写入非法 JSON 会清空 NewAPI 的运行时价格表，所以每次写入前都做格式校验，写入后再读回来核对；售价或站点汇率为 NaN 或 ≤ 0 时校验失败，不调用写入方法。
- 每次写入记录 `price_history`，可以一键回滚。

站点汇率可调：
- 站点汇率改变时，工具重写所有厂商的 `GroupRatio`，同时更新 NewAPI 的 `Price` 和 `USDExchangeRate`。
- 例如从 7 改成 1 时，厂商分组倍率整体放大 7 倍，人民币价格不变。用户已有的余额按美元计价，会被重新估值，界面上要二次确认并提示这一点。
- 站点汇率变化引起的倍率变化不触发"变化保护阈值"（售价本身没变）。

一个模型只能属于一个厂商分组：
- 如果同一个模型名出现在两个厂商分组里，auto 会选 `AutoGroups` 中靠前的组，价格就错了。
- nam 在渠道创建、编辑时校验，并在一致性检查中复查；渠道的 `group` 必须恰好等于所属厂商分组。

偏差检查：
- nam 保存一份原厂价库（只用于成本计算，只涉及站点上存在的模型），并拿它与 NewAPI 当前生效的模型价格比较。
- 偏差超过 5% 时，在"模型与价格"页给出警告，**不自动写入**。
- 把某个模型价格写入 NewAPI 只能手动操作（按建议执行），先预览再确认。
- M0 ④ 实测 NewAPI 实际生效的模型价格（包括硬编码的 `CompletionRatio`）是否等于原厂价，用来校准这个警告。

已知限制与以后：
- 不使用 auto 的令牌会绕过这套机制；nam 不开启 NewAPI 的"默认使用 auto 分组"，由用户自己选 auto；令牌页上能看到厂商分组名（仅展示）。
- 用户分组加价以后做：nam 自动生成 `GroupGroupRatio`（用户分组 × 厂商分组）。现在的方案不会阻碍它；设置了 `GroupGroupRatio` 时 NewAPI 优先用它计费。

为模式 1 预留：`stats_hourly` 保存了渠道级别的成本。以后做模式 1 时，在 NewAPI 的结算阶段按实际渠道成本重算即可，现在的数据结构不需要改。

## 9. 待讨论：优先级与故障转移（只留接口）
- 工具侧预留 `Scorer` 接口：输入渠道统计、成本和余额，输出每个渠道的 priority 和 weight。第一版只实现两种：手动指定，或按成本简单分层。
- NewAPI 侧可能需要的补丁（P3），需要讨论后再决定：
  - 能否按模型单独设置优先级。现在 abilities 表的优先级会在每次保存渠道时被渠道级的值覆盖。
  - 错误分类：哪些错误永久禁用，哪些只临时冷却。
  - 冷却和熔断的时长。
  - 重试次数，以及失败后是整层降级还是逐个渠道降级（`UseNextPriorityOnFailure`）。
  - 首字超时（`response_timeout`，已有）。
  - 缓存亲和。
- 讨论议题清单（每个议题单独一轮）：
  1. 成本和质量的取舍，以及分层宽度。
  2. 质量指标的权重和统计窗口。
  3. 探索流量的比例。
  4. 防抖的阈值。
  5. 缓存亲和的策略（所有客户端都会用到）。
  6. 错误分类表。
  7. 冷却的时长曲线。
  8. 恢复测试的节奏。

## 10. 界面
- 技术栈：React 18 + TypeScript + Vite + Tailwind CSS + shadcn/ui（基于 Radix）+ TanStack Table。图表用 shadcn chart（Recharts），日期选择用 shadcn Calendar（react-day-picker），Toast 用 sonner，Ctrl+K 搜索用 shadcn Command（cmdk）。**不使用 MUI**。
- **UI 从零重新实现**：不参考 `docs/ui/mockup.html` 和 `temp/ui-src` 的代码，也不沿用它们的视觉风格（两者只作为历史保留，已过时，不作为实现依据）。功能需求以 `docs/ui-features.md` 为准；视觉设计重做，但仍要足够紧凑，能放下可编辑的大表格。不使用 Material Design 的外观。
- 渠道表格由 TanStack Table 负责分组、折叠、列显隐、排序，单元格选区和编辑自己实现：可以复用 oc-modal-configurator 的 `selection/machine.ts` 和 `useSpreadsheetSelection.ts`（纯逻辑，只依赖 React，UI 不复用），提供框选、Ctrl+C / Ctrl+V 和 Esc。
- 独立页面，不修改 NewAPI 现有前端，不用 iframe。构建产物由 nam 核心 embed，挂在 `/nam/`（Vite `base: '/nam/'`）。
- nam 的所有 HTTP 接口统一使用 `/api/nam/` 前缀，两种模式相同（独立模式也注册在 `/api/nam/` 下）。§11.1 中所有 nam 接口都按这个完整路径书写；不带 `/api/nam/` 的 `/api/...` 路径指 NewAPI 自己的接口。
- API 客户端统一从 `/api/nam/*` 调用。嵌入模式下请求带 NewAPI 的 session cookie，以及从 localStorage `user.id` 读出的 `New-Api-User` 头；独立模式下用 nam 自己的会话。
- 页面：
  - 上游：表格，可按类型和标签筛选；显示余额、上游货币汇率、最近同步时间和错误。
  - 渠道：表格编辑，支持按上游、厂商、标签分组折叠；一行一个渠道，列包括厂商、令牌、上游分组折扣、模型、priority、weight、状态、成功率、首字延迟、吐字速度、缓存率、成本。厂商决定渠道的分组，不手填。
  - 批量创建：选一个上游，勾选多个令牌，再选厂商，一次生成多个渠道（分组自动取厂商分组）并预览差异。
  - 模型与价格：厂商区显示售价、模式、参考成本价、将写入的分组倍率和待确认状态；模型表列出原厂价、NewAPI 当前价、偏差、渠道数和状态（正常 / 偏差 / 缺价 / 按次计费），支持手动覆盖和别名映射；"预览写入"列出 `GroupRatio` 的变化。
  - 统计看板：按渠道和模型展示趋势，时间范围含最新（最近 100 次请求）、3 天、7 天。
  - 写入历史：变更列表，支持回滚。
  - 设置：站点汇率、厂商分组（分组名、包含的模型、在 `AutoGroups` 中的顺序，写入 NewAPI 前需确认）、通知、调度间隔。具体功能清单见 `docs/ui-features.md`。
- 修改先进入草稿，点"应用"后才写入 NewAPI。应用前会展示将要发出的请求清单。
- 登录：
  - 嵌入模式：使用 NewAPI 的登录，只允许 root；未登录时跳转到 NewAPI 登录页。前端调用 `GET /api/nam/session` 获取模式和身份。
  - 独立模式：单个管理员密码，使用 argon2id 哈希，会话放在 Cookie 里。工具只监听内网或 127.0.0.1，由反向代理对外提供访问。

## 11. 里程碑
| 阶段 | 内容 | 验收（可执行场景见 §11.1） |
|---|---|---|
| M0 实测核实（约 1–2 天） | 先做最小版 mock-upstream（`nam/cmd/mockupstream`：`/v1/chat/completions`、`/v1/messages`、`/v1beta/models/*`、`/_ctl`、`/_calls`，其余接口在后续里程碑补）。在本地 docker 中运行 clansty，逐项确认以下 13 点，结论写入 `docs/m0-findings.md`：①type=58 对 OpenAI、Claude、Gemini 请求的透传；②渠道创建后怎么拿到 ID（创建接口不返回 ID，用唯一名称再搜索）；③PUT 写入零值的行为；④核对 NewAPI 实际生效的模型价格（含硬编码的 `CompletionRatio`）是否等于原厂价，用来校准偏差警告；⑤只测被禁用渠道的行为；⑥非流式请求的 frt；⑦sub2api 的余额和分组接口；⑧价格库字段的映射；⑨`/nam/` 路由不被静态文件和 NoRoute 拦截；⑩生产 SQLite 的 journal_mode；⑪渠道和配置写入的完整副作用；⑫auto 令牌 + 厂商分组：请求某厂商分组 X 内的模型会落到分组 X，日志 `group` 字段为 X，扣费按 `GroupRatio[X]`；⑬独立模式的日志接口是否支持按渠道过滤 | 每项都有请求和响应样例 |
| M1 骨架 | nam 核心模块（四个接口、`nam.db` 迁移、凭证加密、`safeGo`、调度器）；**先做独立适配器**（HTTP 客户端含限速和审计、自带登录）；先写好 `/api/nam/*` 的 OpenAPI 契约，再做前端骨架和表格组件；确定 Go 模块路径 | `go test` 通过；独立模式能读出 NewAPI 的渠道列表；核心模块不引用 new-api 包 |
| M1.5 嵌入 | new-api 仓库中的 `nambridge` 嵌入适配器；`main.go` 启动、路由注册、Dockerfile；`RootAuth`；侧边栏入口（可选）；两种适配器跑同一套接口契约测试 | 嵌入模式下 root 能打开 `/nam/`，管理员和普通用户被拒绝；契约测试两种适配器都通过 |
| M2 上游与渠道 | 上游增删改查；newapi、sk-key、official、generic 四种适配器；令牌同步；批量创建和编辑渠道并写入 NewAPI；一致性检查 | 一个上游下批量建 3 个渠道，NewAPI 中可以调用成功 |
| M3 价格与余额 | 公开价格库（原厂价）同步；上游分组折扣的自动推算和覆盖；余额同步；sub2api 和 onehub 适配器 | 渠道表显示上游分组折扣、余额和成本 |
| M4 统计 | 增量拉取日志；`stats_hourly` 聚合；"最新"档直查 NewAPI 日志；看板；清理原始日志；补丁 P1 | 和 NewAPI 日志抽样对账，误差小于 1% |
| M5 定价 | 厂商售价（固定 / 跟随参考成本价）；写入 `GroupRatio` / `AutoGroups` / `UserUsableGroups`；变化保护与待确认；偏差检查；回滚；站点汇率可调 | 固定和跟随两种模式的分组倍率正确；站点汇率从 7 改为 1 时人民币价格不变 |
| M6 通知 | Telegram、Webhook、邮件；低余额、渠道被禁用、价格变动、公告变化的提醒；去重 | 每类事件都能触发通知 |
| M7 部署 | 嵌入模式：new-api 的 Dockerfile 同时构建 nam 前端；独立模式：nam 自己的 Dockerfile；compose 示例；WSL 构建脚本；备份；README | 嵌入模式在服务器上运行 48 小时，new-api 容器内存比未嵌入时增加不超过 100MB |
| M8 | §9 讨论的结果 | 另写计划 |

### 11.1 QA 场景
#### 共用测试环境
- `deploy/qa/docker-compose.qa.yml` 启动三个服务：
  - `new-api`：待测镜像。
  - `nam`：管理工具。
  - `mock-upstream`：本仓库内的 Go 程序 `nam/cmd/mockupstream`。最小版在 M0 完成，其余接口随后续里程碑补齐。
- mock-upstream 模拟一个 NewAPI 系上游：
  - 实现的接口：`/api/status`（quota_per_unit=500000）、`/api/user/self`、`/api/token/`、`/api/pricing`、`/api/notice`、`/v1/dashboard/billing/*`、`/v1/chat/completions`、`/v1/messages`、`/v1beta/models/*`。
  - 固定返回的用量：prompt 1000（其中 cache_tokens 200）、completion 500。
  - 流式响应首字延迟 300ms，之后 2000ms 内均匀吐完 500 个 token。
  - 控制接口 `POST /_ctl`：可设置下 N 次返回的状态码（429/500/401）、余额、分组倍率、公告内容、延迟。
  - 记录接口 `GET /_calls`：返回每次调用的路径、认证头和所用 key。
  - 通知接收：`/_hook` 记录收到的 Webhook；`/_tg/bot*/sendMessage` 模拟 Telegram，nam 中把 Telegram API Base 指向它。
  - 邮件使用 `mailpit` 容器，通过它的 `GET /api/v1/messages` 统计收到的邮件。
- 原厂价样本 `testdata/prices.json`（单位：美元每百万 token）：`test-model` 输入 3、输出 15、缓存读 0.3、5 分钟缓存写 3.75。
- 单次请求的原厂成本：(800×3 + 200×0.3 + 500×15) ÷ 1e6 = **0.00996 美元**。下文记为**单次原厂成本**。
- 执行方式：`deploy/qa/run.sh M<x>` 启动环境，并执行 `go test -tags qa ./qa/... -run M<x>`，逐条输出 PASS 或 FAIL。断言写在测试代码里。
- 金额断言的误差限：单个值的相对误差 ≤ 1e-6；涉及 NewAPI 整数 quota 的值相对误差 ≤ 1%。

#### M0 实测核实
每项写一个 `docs/m0/<编号>.http` 脚本，执行后把请求、响应和结论写入 `docs/m0-findings.md`。13 项结论齐全即通过。

| 项 | 步骤 | 记录或断言 |
|---|---|---|
| ① | 在 NewAPI 建一个 type=58 渠道指向 mock，分别发 chat、messages、generateContent 三种请求 | `/_calls` 中的路径分别为 `/v1/chat/completions`、`/v1/messages`、`/v1beta/models/test-model:generateContent`；认证头分别为 Bearer、x-api-key、x-goog-api-key |
| ② | `POST /api/channel/`，然后 `GET /api/channel/search?keyword=<唯一名>&id_sort=true` | 创建接口的响应不含 ID；搜索结果中恰好 1 条精确匹配 |
| ③ | `PUT {"id":X,"priority":0}` 后读取；再 `PUT {"id":X,"models":""}` 后读取 | 记录 priority 是否变成 0，models 是否保持不变 |
| ④ | 不写任何价格配置，直接用 `claude-sonnet-4-5` 等几个常用模型各发一次请求 | 记录日志 `other` 中的模型倍率、`completion_ratio`、缓存倍率，与原厂价库对照，判断 NewAPI 实际生效价格是否等于原厂价，据此校准偏差警告的阈值（P2 大概率不需要） |
| ⑤ | 关闭全量自动测试、开启自动恢复；用 `/_ctl` 让渠道返回 401，使其被自动禁用；再把 mock 恢复正常，等待一个测试周期 | 记录渠道状态从 3 恢复为 1 所用的时间；`/_calls` 中只有被禁用渠道的测试请求 |
| ⑥ | 分别发一次非流式和一次流式请求 | 记录两条日志各自的 `frt` 值 |
| ⑦ | 用一个真实的 sub2api 上游测试登录、余额和分组接口 | 记录接口路径和响应字段 |
| ⑧ | 下载 LiteLLM 和 models.dev 的价格数据，对照 20 个常用模型 | 记录字段映射表和缺失项，并选定主数据源 |
| ⑨ | 在 new-api 里注册 `GET /nam/ping` 和 `GET /api/nam/ping`，访问 `/nam/`、`/nam/xxx`（SPA 深链） | `static.Serve("/")` 和 `NoRoute` 不拦截这些路径；记录结果 |
| ⑩ | 查看生产 NewAPI SQLite 的 `PRAGMA journal_mode` | 记录是否为 WAL；不是 WAL 时，评估嵌入模式下写渠道和价格时的锁等待 |
| ⑪ | 阅读并记录 `controller.AddChannel`、`UpdateChannel`、`UpdateOption` 中除写库外的副作用（校验、abilities、缓存刷新、通知） | 列出嵌入适配器必须复用或复现的步骤 |
| ⑫ | 建厂商分组 `anthropic`，写入 `GroupRatio`、`AutoGroups`、`UserUsableGroups`；渠道 `group` 填 `anthropic`；用 auto 令牌请求 test-model | 请求落在 `anthropic` 分组的渠道；日志 `group` 字段为 `anthropic`；扣费 quota 按 `GroupRatio[anthropic]` 计算 |
| ⑬ | 独立模式下调用 `GET /api/log/?channel=<id>&start_timestamp=...` | 记录是否支持按渠道过滤、返回的字段和分页行为 |

#### M1 骨架
（以下第 2–5 步在独立模式下执行；嵌入模式的验收见 M2 第 7 步。）
0. `cd nam && go list -deps ./... | grep QuantumNous/new-api` 无输出，证明核心不引用 new-api。
1. `go test ./...` 退出码为 0。
2. 用错误密码调用 `POST /api/nam/auth/login` 返回 401；用正确密码返回 200 并设置 Cookie；不带 Cookie 访问 `GET /api/nam/upstreams` 返回 401。
3. 在 NewAPI 中预先建 2 个渠道，调用 nam 的 `GET /api/nam/newapi/channels`，返回数量等于 NewAPI `GET /api/channel/` 的 total。
4. 保存一个 key 为 `sk-plaintext-test` 的上游后，用 `sqlite3 nam.db "select hex(cred) from upstream"` 检查，结果中不含该明文的 hex。
5. 单元测试中用 httptest 服务器接收 20 次请求，相邻两次请求的间隔都 ≥ 200ms；每次写入都在 `audit_log` 中有一条记录。

#### M2 上游与渠道
1. 通过 nam API 创建上游 `mock-a`（类型 newapi，Access Token 认证，上游货币汇率 0.5），然后执行令牌同步。mock 预置 3 个令牌（分组 a、b、c）。断言：`upstream_token` 中有 3 行，分组名正确。
2. 调用批量创建接口：上游 mock-a，令牌 a、b、c，厂商 anthropic（模型 `test-model`）。
    - nam 返回 3 个渠道，每个渠道都已关联 NewAPI 渠道 ID。
    - 在 NewAPI 中用 `GET /api/channel/search` 能查到这 3 个渠道，type=58，group=`anthropic`（不是 `self,friend`），models 含 test-model。
3. 用 group 为 `auto` 的令牌向 NewAPI 发 3 次 `POST /v1/chat/completions`，全部返回 200；`/_calls` 中出现的 key 属于这 3 个令牌之一。（前置：`AutoGroups` 含 `anthropic`。）
4. 在 nam 中把 3 个渠道的 priority 批量改为 5，然后点"应用"。断言：NewAPI 中 3 个渠道的 priority 都是 5，`audit_log` 新增 3 条记录。
5. 直接调用 NewAPI 的 `PUT /api/channel/`，把其中一个渠道的 weight 改为 9，触发一致性检查。断言：nam 把这个渠道标记为"漂移"并列出差异字段 weight，NewAPI 中的值没有被自动改回。
6. generic 适配器：配置余额地址 `/_custom/bal`、JSON 路径 `data.left`、换算系数 0.01；mock 返回 `{"data":{"left":12345}}`。断言：余额为 123.45。
7. 双模式：第 2、4、5 步在嵌入模式和独立模式下各执行一次，结果一致。嵌入模式下，用管理员（角色 10）和普通用户的会话访问 `GET /api/nam/upstreams`，均被拒绝；root 返回 200。
8. 一个模型只属于一个厂商分组：尝试把 test-model 加到另一个厂商分组的渠道上，nam 拒绝并给出提示。

#### M3 价格与余额
1. 导入 `testdata/prices.json`，断言 `official_price` 中 test-model 的 4 项原厂价与样本一致。
2. 用 `/_ctl` 把上游 test-model 的模型倍率设为 1.5（原厂模型倍率按输入价 3 计算为 1.5），分组 a 倍率设为 0.4，然后执行倍率同步。
    - 渠道 a 的上游分组折扣 = 0.4，来源为"自动"。
    - 手动把上游分组折扣覆盖为 0.35，再同步一次，仍为 0.35。
3. mock 的余额设为 quota 2500000，执行余额同步，断言余额为 5.00 美元。上游货币汇率 0.5，所以剩余价值显示为 2.50 元。
4. 渠道表接口 `GET /api/nam/channels` 返回渠道 a 的上游分组折扣、余额和单次成本。单次成本应为 0.00996 × 0.35 × 0.5 = 0.001743 元，按 1e-6 相对误差断言。
5. sub2api 和 onehub 适配器用各自的 mock 路由重复第 3 步。如果 M0 ⑦ 发现接口不可用，在 M0 结论中把对应项标为"手动"，本步跳过并记录原因。

#### M4 统计
1. 前置：先清空 NewAPI 的 logs；用 `/_ctl` 让渠道 a 前 2 次请求返回 500，渠道 b 正常。
2. 发 20 次流式请求和 10 次非流式请求，再在 NewAPI 中对渠道 a 执行一次"测试"。
3. 等待一个日志拉取周期（测试中可以手动触发 `POST /api/nam/jobs/pull-logs`）。断言：
   - 测试日志被排除：`stats_hourly` 中所有渠道的成功次数之和 = 30。
   - 渠道 a 的失败次数 = 2，错误类别为 5xx。
   - 首字延迟 p50 落在 [250, 450]ms。
   - 吐字速度的中位数落在 [200, 300] token/s（500 token ÷ 2.0s = 250）。
   - 缓存命中率为 200/1000 = 20%，绝对误差 ≤ 0.1 个百分点。
   - "最新"档：渠道 a 的最近请求列表直接来自 NewAPI 日志（不读 `stats_hourly`），条数 ≤ 100，且不含测试日志。
4. 对账：nam 计算出的总成本与逐条日志按 §7 公式手工复算的结果相比，相对误差 ≤ 1e-6；nam 计算出的收入与 NewAPI 日志 quota 之和 ÷ 500000 × 站点汇率 相比，相对误差 ≤ 1e-6。
5. 把原始日志保留天数设为 0，执行清理任务。断言：NewAPI 的 `GET /api/log/?type=2` 返回 total=0，`stats_hourly` 中的数据不变。
6. 补丁 P1：在 NewAPI 补丁分支中执行 `go test ./service/... ./model/...` 通过；新的日志中出现 `use_time_ms`，取值在 [2200, 2600]ms 之内；非流式日志不再出现 `frt=-1000`。

#### M5 定价
本节按 §8 的厂商分组方案验收。test-model 归属厂商 `anthropic`（与 M2 一致）。

1. 前置（独立构造数据，不依赖 M4 的流量）：
   - 站点汇率 = 7；变化保护阈值 50%；自动写入开启。
   - 测试环境预先设好 NewAPI 的模型价格（nam 不写）：`ModelRatio[test-model]` = 1.5，`CompletionRatio` = 5，`CacheRatio` = 0.1，`CreateCacheRatio` = 1.25。
   - 厂商分组 `anthropic`；渠道 a、b、c 的 group 都是 `anthropic`；`AutoGroups` 含 `anthropic`；测试令牌的 group 为 `auto`。
   - 在 NewAPI 中把渠道 b、c 设为手动禁用（状态 2），只保留渠道 a 启用；渠道 a 的上游分组折扣 0.35，上游货币汇率 0.5，上游渠道成本价 = 0.175。
   - 清空 NewAPI 的 logs 和 nam 的 `stats_hourly`、日志游标；`/_ctl` 重置为全部正常返回。
   - 发 10 次请求，然后调用 `POST /api/nam/jobs/pull-logs`。断言 `stats_hourly` 中只有渠道 a 的记录、成功次数 = 10。
   - 每次请求的单次原厂成本为 0.00996 美元，nam 计算的实际成本为 0.00996 × 0.35 × 0.5，所以 anthropic 的实测成本价 = 0.175。
2. 固定模式：调用 `PUT /api/nam/vendor-pricing/anthropic {"mode":"fixed","sell_price":0.35}`，再调用 `POST /api/nam/jobs/pricing`。断言：
   - `GroupRatio[anthropic]` = 0.35 ÷ 7 = 0.05。
   - `ModelRatio`、`CompletionRatio`、`CacheRatio`、`CreateCacheRatio` 保持第 1 步的值不变（nam 不写模型价格）。
   - 写入后读回的值与上述一致，`price_history` 新增 1 条记录。
   - 发 1 次请求：日志 quota ÷ 500000 × 7 = 0.00996 × 0.35 = 0.003486 元，误差 ≤ 1%（NewAPI 的 quota 是整数）。
3. 跟随参考成本模式：调用 `PUT /api/nam/vendor-pricing/anthropic {"mode":"follow","markup":1.5}`，再执行定价。断言：
   - `GET /api/nam/vendor-pricing/anthropic` 返回参考成本价 0.175，售价 = 0.175 × 1.5 = 0.2625。
   - 售价从 0.35 变为 0.2625（−25%），没有超过阈值，直接写入。`GroupRatio[anthropic]` = 0.2625 ÷ 7 = 0.0375。
4. 修改站点汇率：调用 `PUT /api/nam/settings {"site_rate":1}`（带二次确认参数）。断言：
   - `GroupRatio[anthropic]` = 0.2625 ÷ 1 = 0.2625，即原值的 7 倍。
   - NewAPI 中 `Price` 和 `USDExchangeRate` 都等于 1。
   - 没有产生"待确认"。
   - 再发 1 次请求，quota ÷ 500000 × 1 = 0.00996 × 0.2625 元，与站点汇率为 7 时按同一售价计算的人民币费用相比，相对误差 ≤ 1%。
5. 保护措施：
   - 清空 NewAPI 的 logs 和 nam 的 `stats_hourly`、日志游标（渠道 b、c 仍为禁用），把渠道 a 的上游分组折扣改为 0.7，发 10 次请求并调用 `POST /api/nam/jobs/pull-logs`，然后执行定价。参考成本价变为 0.35，售价变为 0.525（+100%），超过阈值。断言：`GroupRatio[anthropic]` 仍为 0.2625，anthropic 状态为"待确认"，产生一条"价格变动待确认"通知。
   - 调用 `POST /api/nam/vendor-pricing/anthropic/confirm`。断言：`GroupRatio[anthropic]` = 0.525，`price_history` 新增 1 条记录。
   - 删除 test-model 的原厂价后执行定价（跟随模式）。断言：该模型无法计入参考成本价，状态为"缺价"，产生一条告警，`GroupRatio` 不变。
6. 偏差检查：在 NewAPI 中手动把 `ModelRatio[test-model]` 改为 2.0。断言：nam 的"模型与价格"接口显示该模型状态为"偏差"（约 +33%），且 nam 没有写 `ModelRatio`。
7. 回滚：对第 5 步"确认"产生的那条 `price_history` 调用 `POST /api/nam/price-history/<id>/rollback`。断言：NewAPI 中的 `GroupRatio[anthropic]` 恢复为 0.2625，读回核对一致，回滚本身新增一条审计记录。
8. 注入非法值：在单元测试中让写入函数收到 NaN，或售价 ≤ 0，或站点汇率 ≤ 0。断言：校验失败，不调用 `OptionStore` 的写入方法。
9. 双模式：第 2、4 步在嵌入模式和独立模式下各执行一次，写入的 `GroupRatio` 一致。

#### M6 通知
前置：配置 Webhook 指向 `/_hook`，Telegram 指向 mock，邮件发往 mailpit，冷静期 300 秒。

| 事件 | 触发方式 | 断言 |
|---|---|---|
| 低余额 | 阈值设为 10 元，mock 余额设为 1 元，执行余额同步 | 三个通道各收到 1 条通知，内容含上游名称 |
| 渠道被禁用 | 用 `/_ctl` 返回 401，发请求使 NewAPI 自动禁用该渠道，等待一致性检查 | 三个通道各收到 1 条通知 |
| 价格变动 | 修改 mock 中的分组倍率，执行倍率同步 | 收到通知，内容含新旧上游分组折扣 |
| 公告变化 | 修改 mock 的 `/api/notice` 内容，执行公告检查 | 收到通知；再执行一次检查，不再产生新通知 |
| 去重 | 低余额事件在 300 秒内触发 3 次 | 每个通道总共只收到 1 条 |

#### M7 部署
1. 在 WSL 中执行 `deploy/build-wsl.sh`，生成 `nam.tar.gz` 和 `new-api-nam.tar.gz`，脚本退出码为 0。
2. 在服务器上 `docker load` 两个镜像，然后 `docker compose up -d`。断言：
   - `curl -fsS http://127.0.0.1:<nam端口>/healthz` 返回 `ok`。
   - nam 能读到 NewAPI 的渠道列表。
3. 内存采样：部署后运行 `deploy/qa/memwatch.sh`，每 5 分钟执行一次 `docker stats --no-stream --format '{{.MemUsage}}' new-api`（独立模式改为 `nam`），结果追加到 `memwatch.csv`，持续 48 小时。断言：嵌入模式比未嵌入时的基线增加 ≤ 100MiB（独立模式最大值 ≤ 100MiB），并且最后 6 小时的均值不超过前 6 小时均值的 1.2 倍，用来排查内存泄漏。
4. 备份：执行备份任务，生成 `backup/nam-<日期>.db`；把它恢复到一个新容器中，`GET /api/nam/upstreams` 返回的数量与原实例一致。
5. 回滚：把 compose 中的 new-api 镜像标签改回上一个版本，执行 `up -d`，NewAPI 正常响应 `/api/status`。

## 12. 风险与对策
| 风险 | 对策 |
|---|---|
| NewAPI 写入非法 JSON 会清空运行时价格表（现在 nam 只写 `GroupRatio`、`AutoGroups`、`UserUsableGroups`、`Price`、`USDExchangeRate`） | 写入前校验，写入后读回核对；出错时立即回滚 |
| 同一个模型名出现在两个厂商分组里，auto 会选 `AutoGroups` 中靠前的组，价格算错 | 渠道创建、编辑时校验，一致性检查中复查；渠道的 `group` 必须恰好等于所属厂商分组 |
| 不用 auto 的令牌会绕过厂商分组倍率机制 | nam 不开启 NewAPI 的"默认使用 auto 分组"；告知用户令牌选 auto；令牌页的厂商分组名仅作展示；已记为已知限制 |
| NewAPI 实际生效的模型价格与原厂价不一致（例如硬编码的 `CompletionRatio`） | 偏差检查超过 5% 在"模型与价格"页警告，不自动写入；写模型价格只能手动、先预览再确认；M0 ④ 实测校准 |
| 创建渠道的接口不返回 ID | 渠道名称使用 `nam-<上游>-<令牌>-<厂商>-<短 ID>`，创建后搜索精确匹配；之后用 NewAPI 的 ID 作为主键 |
| PUT 不能把非指针字段清零（例如 status=0、models 为空） | 改状态用 1、2、3；需要清空模型时改为删除渠道再重建 |
| 渠道的 group 字段只有 varchar(64) | 厂商分组名保持简短（例如 claude、openai、gemini） |
| 公开价格库缺失模型或缺缓存价 | 用手动覆盖和别名映射；缺价的模型不计入参考成本价并告警 |
| 上游改版导致适配器失效 | 每个适配器有错误计数；连续失败就告警并停止自动更新相关的上游分组折扣 |
| 1c2g 资源紧张 | 所有任务串行；日志分页拉取；原始日志只保留 30 天 |
| 站点汇率改变后，用户已有余额被重新估值 | 界面二次确认，并提示影响 |
| clansty 更新后补丁冲突 | 补丁尽量少；每个补丁一个提交并带测试；集成改动集中在 `main.go` 一行和路由注册 |
| clansty 改了 model/controller 的内部接口，嵌入适配器编译失败 | 影响只限于 `nambridge`（几百行）；先切到独立模式继续用，再修适配器；接口契约测试覆盖两种适配器 |
| nam 后台任务 panic 拖垮网关 | 所有 goroutine 经过 `safeGo`；代码审查和 lint 检查禁止裸 `go func` |
| 嵌入模式与 NewAPI 抢 SQLite 写锁 | nam 数据只写 `nam.db`；写 NewAPI 只有渠道和配置等低频操作，用小事务；M0 ⑩ 确认 journal_mode |
| 嵌入后依赖版本冲突 | 核心模块尽量少引依赖；gorm、sqlite 驱动、gin 版本与 NewAPI 对齐 |
| 官方的 JWT 登录改造被 clansty 合并 | 嵌入适配器的 `Auth` 换成新中间件；独立适配器改用新的令牌方式；核心不受影响 |
| 非 root 用户访问 nam | 所有接口 `RootAuth()`；前端入口只对 root 显示；QA 覆盖管理员和普通用户被拒绝的场景 |

## 13. 仓库结构（建议）
```
newapi-configurator/
  nam/            nam 核心 Go 模块（不引用 new-api）
    core/         对外入口：New(Config, Ports) → http.Handler + Start/Stop
    ports/        ChannelStore / OptionStore / LogSource / Auth 接口与契约测试
    internal/     store(nam.db), upstream(适配器), pricing, stats, notify, scheduler, audit, crypto
    standalone/   独立适配器（NewAPI HTTP 客户端 + 自带登录）
    cmd/nam/      独立模式的 main
    cmd/mockupstream/  QA 用的 mock 上游（最小版在 M0）
    webdist/      前端构建产物（go:embed）
  web/            React + TS 前端（base: /nam/）
  newapi-bridge/  嵌入适配器源码（nambridge），同步到 new-api 的 nam-patches 分支
  patches/        说明 nam-patches 分支，或存放补丁文件
  deploy/         Dockerfile、docker-compose.example.yml、build-wsl.sh
  docs/           m0-findings.md、api 说明
  temp/           调研用的源码（不提交）
```
- new-api 的 `nam-patches` 分支：功能补丁（P1 等）+ `nambridge/` + `main.go` 一行 + `go.mod` 引用 nam 模块（本地构建时用 `replace` 指向本仓库）。

### 13.1 工程细节
- **Go 模块路径**：在 M1 确定；new-api 通过 `replace` 引用它。
- **`nam.db` 路径**：环境变量 `NAM_DB_PATH`，嵌入模式默认 `/data/nam.db`（NewAPI 的数据卷）。
- **接口契约**：M1 里先写好 `/api/nam/*` 的 OpenAPI 契约，再做前端，前后端都按它对齐。
- **依赖版本固定**（与 new-api 对齐）：gin v1.9.1、gorm v1.25.2、glebarez/sqlite v1.9.0、Go 1.25.x（new-api 的 `go.mod` 写的是 1.25.1）。
- **开发环境**：Windows 上开发（Go 1.25.x 在 `D:\Go`，缓存也在该目录；Node 24）；WSL 的 docker（29.1.3）运行 NewAPI、QA 环境和镜像构建。Windows 上的 nam 通过 `localhost` 访问 WSL docker 里的 NewAPI。
- **mock 上游**：`nam/cmd/mockupstream`，M0 先做最小版，后续里程碑补齐。
- **许可**：clansty 的 new-api 是 AGPL-3.0，嵌入 nam 后合为一个程序；自用没问题，对外发布或提供给别人用要履行 AGPL 义务。

