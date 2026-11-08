# nam 开发说明

本文给接手的人看：代码在哪、怎么跑、怎么测、下一步做什么。架构和需求仍以 `docs/README.md`、计划文件为准。

## 代码结构

### 仓库布局

- **`nam/`：nam 核心 Go 模块**（`github.com/steven52880/nam`）
    - `ports/`：四个接口（ChannelStore、OptionStore、LogSource、Auth）加 Backend  
      `ports/portstest/` 是内存假实现和**契约测试**，两个适配器都必须跑通。
    - `core/`：对外入口。`core.New(Config, Ports)` → `Start(ctx)` → `Mount(gin)` 或 `Handler()`。
    - `internal/store`：`nam.db` 全部表（`models.go`）和仓储函数。
    - `internal/crypto`：AES-GCM 凭证加密，主密钥来自 `NAM_SECRET`。
    - `internal/safego`：**唯一允许**的起 goroutine 方式，带 recover。
    - `internal/scheduler`：串行任务调度器，任务名见 `scheduler.Job*`。
    - `internal/audit`：审计记录，并包装 ChannelStore / OptionStore，<u>所有写 NewAPI 的操作自动记审计</u>。
    - `internal/api`：`/api/nam/*` 处理函数，契约见 `docs/api/openapi.yaml`。
    - `internal/httpx`：带最小间隔的 JSON HTTP 客户端。
    - `standalone/`：独立适配器（HTTP 调 NewAPI 管理 API + 自带登录）。
    - `cmd/nam/`：独立模式入口。`cmd/mockupstream/`：QA 用的 mock 上游。
    - `webdist/`：前端构建产物的 `go:embed`，目录 `dist/` 只提交占位文件。
- **`web/`：前端**（React 18 + Vite + Tailwind + shadcn 风格 + TanStack Table）  
  `npm run build` 直接输出到 `nam/webdist/dist`。
- **`newapi-bridge/nambridge/`：嵌入适配器源码**  
  用 `newapi-bridge/sync.ps1` 复制到 `temp/clansty-new-api/nambridge/` 再编译。
- **`temp/clansty-new-api`**：clansty 源码，分支 `nam-patches` 上已有两个提交（嵌入适配器、非 root 返回 403）。
- **`docs/m0-findings.md`、`docs/m0/*.ps1`**：M0 实测结论和脚本。

### 已定的实现约定

- **核心不引用 new-api**：`go list -deps ./... | Select-String QuantumNous` 必须无输出。
- **不写裸 `go func`**：一律 `safego.Go` / `safego.GoWG`；嵌入适配器里因为不能引用 internal 包，用本地 recover。
- **写 NewAPI 必须经过 `core` 包装后的 Ports**，这样审计不会漏。
- **接口统一信封** `{success, message, data}`，失败时用 HTTP 状态码区分：400 参数、401 未登录、403 非 root、404、409、429 锁定、502 NewAPI 不可达。
- **嵌入模式非 root**：NewAPI 自己返回 200 + success=false，nambridge 把它改写为 **403**。

## 本地运行

### 环境变量（Windows PowerShell）

- 每个新终端先执行：  
  `$env:GOPATH='D:\Go\gopath'; $env:GOMODCACHE='D:\Go\gopath\pkg\mod'; $env:GOCACHE='D:\Go\cache'; $env:PATH="D:\Go\bin;$env:PATH"`
- 下载依赖需要代理：`$env:HTTPS_PROXY='http://127.0.0.1:7890'`
- 用户级环境变量已经写好，**新开的终端**会自动带上前三项。

### 测试

- **nam 核心**：在 `nam/` 下 `go vet ./...`、`go test ./...`（约 10 秒）。
- **嵌入适配器**：先跑 `newapi-bridge/sync.ps1`，再在 `temp/clansty-new-api` 下 `go test ./nambridge/`。
- **前端**：在 `web/` 下 `npm run build`（同时做类型检查）。

### 嵌入模式手动试用

- 在 `temp/clansty-new-api` 下 `go build -o ..\e2e\new-api-nam.exe .`
- 运行前设置：`PORT`、`SQLITE_PATH`、`NAM_SECRET`、`NAM_DB_PATH`。<u>不设 `NAM_SECRET` 时 nam 不挂载，NewAPI 照常运行</u>。
- 浏览器打开 `http://127.0.0.1:<PORT>/nam/`，用 NewAPI 的 root 账号登录。

### 独立模式手动试用

- 在 `nam/` 下 `go build -o nam.exe ./cmd/nam`
- 需要：`NAM_NEWAPI_URL`、`NAM_NEWAPI_TOKEN`（root 的系统访问令牌）、`NAM_NEWAPI_USER_ID`、`NAM_SECRET`、`NAM_ADMIN_PASSWORD`（首次启动的管理员密码）。
- 默认监听 `127.0.0.1:8090`；前端开发时 `web/` 下 `npm run dev`，`/api` 会代理到这里。

## 当前进度

### 已完成并验证

- **M0**：除 ⑦（sub2api，等账号）外全部有结论，⑨ 已用嵌入二进制补测。
- **M1 骨架**
    - 核心模块、`nam.db` 迁移、凭证加密、safego、调度器、审计、`/api/nam` 基础接口。
    - 独立适配器 + 登录（argon2id、5 次失败锁 60 秒、会话 7 天、改密码后会话全部失效）。
    - 前端骨架：外壳、登录、总览、上游、审计、设置，表格组件 `/dev/grid`。
    - **M1 QA 0–5 全部通过**：依赖隔离、`go test`、登录 401/200、独立模式读到的渠道数与 NewAPI 一致、凭证不以明文入库、20 次请求间隔 ≥200ms 且写入有审计。
- **M1.5 嵌入**
    - 契约测试两个适配器都通过。
    - 本地二进制实测：root 能打开 `/nam/` 并读到渠道和任务；管理员和普通用户被拒绝（403）。

- **M2 上游与渠道**
    - **渠道层**（`internal/channels`）：草稿合并与剪枝、校验（厂商决定分组、一个模型只属于一个厂商、名称唯一、折扣大于 0）、串行"应用变更"（失败保留草稿）、漂移检测（自动禁用不算漂移）、批量创建（按厂商拆分渠道）。
    - **渠道类型由厂商决定**：Gemini 厂商用 type=24，其余用 58（第 19 轮）。
    - **上游适配器**：newapi、sk-key、official、generic；令牌、分组倍率、余额同步；向导的五项连通性检查。
    - **厂商**：增删改、排序；写 NewAPI 分组倍率留到 M5。
    - **前端**：渠道表格页（编辑、复制粘贴、右键菜单、模型明细）、应用变更弹窗、漂移对比、批量创建向导、新增上游 5 步向导、上游抽屉各标签页、设置页的厂商分组。
    - **M2 QA**：接口级测试覆盖计划 §11.1 M2 全流程；另用真实 NewAPI 二进制 + mock 上游跑了 12 项端到端检查（`temp/e2e/m2.ps1`），**全部通过**，包括 auto 令牌经 NewAPI 落到新建渠道。

### 下一步要处理的事

- **M3 价格与余额**：原厂价库同步（主源 LiteLLM）、上游分组折扣自动推算与覆盖、sub2api / onehub 适配器。
- **前端**：还没在浏览器里完整点过一遍，<u>建议主人先手动试用渠道页、批量创建和应用变更</u>；包体积约 650 kB，后续拆分。
- **已知问题**
    - 未带日期的 Claude 别名（如 `claude-sonnet-4-5`）在 NewAPI 里没有价格，请求会 400，M3 的别名映射要处理。
    - "在 NewAPI 中打开"用的路径 `/console/channel?id=` 取自 NewAPI 前端源码，没在浏览器验证。
    - 密码方式登录上游时，如果对方开了 2FA 或 Turnstile 会失败。
