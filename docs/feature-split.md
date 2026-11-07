# 功能划分：NewAPI 补丁、NewAPI 配置与 nam

依据：`docs/requirements.md` §4.9（第 13–16 轮的架构决定），以及对 clansty 分支（d292f5e8）的源码调研。

**划分原则**

- **请求路径上**每次都必须执行的逻辑，比如计费、选渠道、重试、冷却，只能写进 NewAPI 补丁。
- 可以**离线算好再写回**的逻辑，一律放进 nam 核心。
- nam 核心**不引用任何 new-api 的包**。和 NewAPI 打交道的部分全部放在适配器里，<u>同一份核心代码可以嵌入运行，也可以独立运行</u>。
- 补丁越少越好，每个补丁一个提交并带测试，方便跟 clansty 一起升级。

## 运行方式

### 嵌入模式（生产默认）

- nam 编译进 NewAPI 的二进制，和网关在同一个进程里跑。
- **数据**
    - 渠道、日志、价格直接用 NewAPI 的数据，<u>只有一份</u>。
    - nam 自己的数据（上游、凭证、统计聚合、审计、草稿）单独存在 `nam.db`。
- **登录**：直接用 NewAPI 的登录会话。所有 `/api/nam/*` 接口都套 `RootAuth()`，只有 root（角色 100）能用。
- **前端**：独立页面，同域挂在 `/nam/` 下。NewAPI 侧边栏放一个只对 root 显示的入口。

### 独立模式（可选）

- nam 单独运行，通过 NewAPI 的管理 API 读写。需要填 NewAPI 地址、root 的系统访问令牌和用户 ID。
- 自带登录页，界面和功能与嵌入模式相同。
- 用途：
    - 本地开发，连真实的 NewAPI 调试，不用每次都重编、重启网关。
    - NewAPI 升级后，嵌入适配器暂时编译不过时，可以先切过来继续用。
    - 给使用原版 NewAPI 的人用。

## 需要修改 NewAPI 代码

### 功能补丁

- **P1：毫秒级耗时（确定要做，约 20 行）**  
  消费日志的 `other` 里增加 `use_time_ms`；非流式请求不再写 `frt=-1000`。  
  `use_time` 只有整秒，算出的吐字速度误差太大。两种运行模式都需要这个补丁。
- **P2：配置表优先于硬编码表（<u>大概率不需要</u>）**  
  让 `CompletionRatio`、`CacheRatio` 配置表的值优先于代码里写死的值。  
  新方案下 nam 不再写模型价格，模型价格保持 **原厂价**。M0 ④ 只用来确认 NewAPI 实际生效的模型价格和原厂价是否一致。
- **P4：日志增量接口（可选，只对独立模式有用）**  
  `GET /api/log/` 支持 `min_id`，并可跳过 COUNT。嵌入模式直接读日志表，不需要这个补丁。
- **P3a–P3d：优先级与故障转移（待讨论）**  
  按模型设置优先级、临时冷却、熔断与重试、缓存亲和。这些发生在请求路径上，只能改 NewAPI。
- **F1：模式 1 计费（以后）**  
  按选中渠道的实际成本重新计价。本期不做，nam 会准备好所需数据。

### 集成改动（嵌入模式需要）

- **`main.go`**：加 1–2 行启动 nam，比如 `nambridge.Start(server)`。
- **路由**：注册 `/api/nam/*` 和 `/nam/*`。显式路由优先于 `NoRoute`，<u>不用改 `web-router.go`</u>。`static.Serve("/")` 不拦截 `/nam/` 这一点需要实测确认。
- **`go.mod`**：引入 nam 核心模块。
- **Dockerfile**：增加 nam 前端的构建步骤。
- **侧边栏入口（可选，约 10 行）**：只对 root 显示。
- **不改的地方**：NewAPI 现有的 `migrateDB` 列表和现有页面都不动。

## 只需改 NewAPI 配置

- **错误日志**：设环境变量 `ERROR_LOG_ENABLED=true`，部署时手动设置。
- **渠道类型**：统一用 type=58 PassThrough，base_url 填上游根地址。nam 创建渠道时自动填写。
- **厂商分组**
    - 每个厂商对应 NewAPI 里的**一个分组**（如 `claude`、`openai`、`gemini`）。
    - 该厂商所有渠道的 group 就是这个分组，由 nam 创建渠道时自动填写；令牌使用 `auto`。
    - nam 写入 `GroupRatio`（= 售价 ÷ 站点汇率）、`AutoGroups`、`UserUsableGroups`。
    - nam **不开启** NewAPI 的"默认使用 auto 分组"，用户自己选 `auto`。
    - 模型价格（ModelRatio 等）保持 **原厂价**，由 NewAPI 自带的价格同步维护，nam 不自动写。
    - 哪个用户属于哪个用户分组，在 NewAPI 后台操作；用户分组加价以后再做。
- **站点汇率**：`Price` 和 `USDExchangeRate` 等于站点汇率，`QuotaPerUnit` 保持不变，由 nam 写入。
- **自动禁用和自动恢复**：`AutomaticDisableChannelEnabled`、`AutomaticEnableChannelEnabled`，关闭全量定时测试。nam 的"设置"页提供开关。
- **渠道测试模型**：每个渠道写 `test_model`，nam 自动写入。
- **朋友的权限和额度**：在 NewAPI 后台手动管理，不在本期范围。

## nam 核心

所有业务逻辑都在核心里，两种运行模式共用。

- **上游管理**：增删改上游，记录类型、凭证（AES-GCM 加密）、上游货币汇率和标签；自动识别站点类型；连通性测试。
- **上游适配器**：newapi、sk-key、sub2api、onehub/donehub、official、generic 六种。读取余额、令牌、分组倍率、模型倍率、逐模型价格和公告。
- **令牌**：从上游同步令牌和所属分组；手动录入；对方支持时可在上游新建令牌。
- **渠道扩展信息**
    - <u>nam 不复制渠道字段</u>，只按 `channel_id` 保存上游、令牌、上游分组折扣、折扣来源和模型级覆盖。
    - 支持批量创建，以及表格批量编辑（框选、复制粘贴）。修改先进草稿，再统一应用。
    - 只检查 nam 管理的字段有没有在 NewAPI 侧被改过（漂移）。
- **原厂价**：同步 LiteLLM 或 models.dev，支持手动覆盖和别名映射。只用于算成本和偏差检查，只覆盖站内存在的模型。
- **渠道成本**：自动推算上游分组折扣；按模型覆盖；计算上游渠道成本价、实测成本价和标准负载成本价。
- **余额**：定时同步，折合成人民币，低余额告警。
- **统计**：按日志 id 游标增量读取，按小时聚合到 `nam.db`，算出成功率、首字延迟、吐字速度、缓存率、成本和收入。
- **定价**
    - 每个厂商设一个**售价**（元 / 原厂 1 美元），写成该厂商的分组倍率。
    - 偏差检查：NewAPI 当前模型价格和原厂价相差超过 5% 时只提示，不自动写。
    - 有变化保护和回滚；站点汇率可调。
    - 用户分组加价：<u>以后</u>再做。
- **优先级（Scorer）**：第一版只做手动指定和按成本分层，接口留给 §9 的算法。
- **通知**：Telegram、Webhook、邮件；同一事件去重。
- **运维**：审计日志、回滚、清理 NewAPI 原始日志、备份 `nam.db`、健康检查。
- **前端**：React + TypeScript + shadcn/ui + Tailwind + TanStack Table，界面**从零重新实现**，只依据 `docs/ui-features.md` 的功能清单，视觉不沿用原型；构建后由核心模块 embed，挂在 `/nam/` 下。

## 适配器

核心通过四个接口和外部交互：

- **ChannelStore**：列出、读取、增删改、批量操作、测试渠道。
- **OptionStore**：读写 ModelRatio、CompletionRatio、CacheRatio、GroupRatio、Price 等配置项。
- **LogSource**：按 id 游标读取消费日志和错误日志，读取后可清理；**按渠道读取最近 N 条**（统计的"最新"档直接查 NewAPI 日志，不另存样本）。
- **Auth**：判断当前请求是否来自 root。

### 嵌入适配器

- 放在 new-api 仓库里，比如 `nambridge/` 目录，跟着 NewAPI 一起编译。
- **渠道**：复用 controller 中 `AddChannel`、`UpdateChannel` 的完整流程（校验、abilities），写完调 `model.InitChannelCache()`。
- **配置项**：走 `model.UpdateOption`，写完调 `model.InvalidatePricingCache()`。
- **日志**：直接查 `LOG_DB` 的 `logs` 表，用 `id` 主键做游标。按渠道读最近 N 条时，用 `channel_id` 加时间范围、`id` 倒序取前 N 条，只选需要的列。<u>不调用 `GetLogPerformance`</u>，它会把整个时间段的日志一次读进内存。
- **身份**：`middleware.RootAuth()`；高危操作（查看凭证明文、修改站点汇率）再加 `SecureVerificationRequired()`。
- **硬性要求**
    - nam 的所有后台 goroutine 都要经过统一的 recover 包装。gin 的 recovery 只保护请求处理函数，后台 panic 会拖垮网关。
    - 关闭时跟随 `common.ShutdownCtx()` 退出。

### 独立适配器

- 放在 nam 仓库里，通过 HTTP 调用 NewAPI 管理 API，带上 root 访问令牌和 `New-Api-User` 请求头。
- **渠道**：`/api/channel/*`。
- **配置项**：`/api/option/`。
- **日志**：`GET /api/log/`，必须带 `start_timestamp`，按 id 去重。按渠道读最近 N 条依赖该接口的渠道筛选（M0 ⑬ 确认）。
- **身份**：nam 自己的登录页。"测试连接"时调 `/api/user/self`，确认 role 是 100。
- 访问 NewAPI 的请求间隔不小于 200ms。
