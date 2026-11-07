# New API 上游与 clansty 分支技术调查报告

`QuantumNous/new-api` 与 `clansty/new-api` 并非两款平行演进的同类产品。`clansty/new-api` 创建于 2026-03-27，与上游的最后共同祖先停在 2026-04-27（`bee339d279ccecbf8c8a89e14ddbbd902f78bd5d`，对应 `v0.13.2` 时期）。自该节点起，上游 `main` 推进了 995 个提交（HEAD `ba5f7367797566175a6cd97a748d430af3602a01`），完成了通往 `v1.0.0-rc.47` 的网关架构重写；分支则在默认分支 `clansty` 上推进了 155 个提交（HEAD `d292f5e8a0cb0b70ca72bdef8c02adc09a70dde0`），几乎全部由单一维护者 Clansty 围绕自营生产环境编写，并将界面中的 OIDC 登录品牌统一改为 MuNET。分支仓库中的 `main` 分支停留在 2026-03-27 的快照（`fbf235d2`），相对当前上游落后 1191 个提交、超前 0 个提交，不包含任何分支自有代码。

---

## 一、调查方法与边界

本报告的数据采集截止于 2026-11-04 上午（上海时间，Asia/Shanghai）。所有结论基于 GitHub REST API 元数据以及两份仓库的本地克隆对比（工作区检出两个 HEAD 的源码、目录树、依赖清单与测试文件，提交历史用 git 对称差核对）。

本次调查只做静态代码与提交历史核查：未执行任何单元测试或集成测试，未编译或启动任一版本的服务进程，未连接数据库，也未访问或登录任何线上部署实例。报告中凡未逐文件核对到底层实现的线索，均明确标为“不确定”或归入“只见于提交说明”，不做推演。

---

## 二、时间线、仓库身份与分叉几何

### 1. 仓库基本身份对照

| 维度 | 上游 `QuantumNous/new-api` | 分支 `clansty/new-api` |
|---|---|---|
| 仓库地址 | `https://github.com/QuantumNous/new-api` | `https://github.com/clansty/new-api` |
| 派生关系 | 源仓库（非 fork） | fork 自 `QuantumNous/new-api` |
| 创建时间（上海） | 2023-11-11 03:33（`2023-11-10T19:33:36Z`） | 2026-03-27 21:57（`2026-03-27T13:57:34Z`） |
| 最近推送 `pushed_at`（上海） | 2026-11-03 19:33（`2026-11-03T11:33:43Z`） | 2026-10-18 19:09（`2026-10-18T11:09:29Z`） |
| 元数据更新 `updated_at`（上海） | 2026-11-04 09:56（`2026-11-04T01:56:12Z`） | 2026-10-18 19:09（`2026-10-18T11:09:29Z`） |
| 默认分支 | `main` | `clansty` |
| 默认分支 HEAD SHA | `ba5f7367797566175a6cd97a748d430af3602a01` | `d292f5e8a0cb0b70ca72bdef8c02adc09a70dde0` |
| HEAD 提交时间与作者 | 2026-11-03 19:33 +0800，Seefs | 2026-10-18 18:44:03 +0800，Clansty |
| HEAD 提交说明 | `fix(gemini): bill embeddings from upstream usageMetadata (#8182)` | `fix: 图像响应无图片输出时不再按成功结算` |
| Star / Fork / Open Issues / Subscribers | 51295 / 12456 / 1421 / 159 | 1 / 0 / 0 / 0 |
| 许可证 | GNU AGPL-3.0 | GNU AGPL-3.0 |
| GitHub 仓库体积 | 92795 KB | 43718 KB |
| 最新 Release | `v1.0.0-rc.47`（2026-10-25 17:04 上海） | 无 Release |
| 主要贡献者（GitHub API 计数） | Calcium-Ion (3098)、seefs001 (632)、songquanpeng (509)、t0ng7u (468)、feitianbubu (226) | clansty (157)；独有提交中 Clansty 154、akachan 1 |

上游拥有约 9 个分支，并在 2026-09-21 至 2026-10-25 期间密集发布了 `v1.0.0-rc.40` 至 `v1.0.0-rc.47`（其中 `rc.40` 为 `0aec08fe`，2026-09-21；`rc.41` 为 `2035a82a`，09-30；`rc.42` 为 `6370b294`，10-08；`rc.43` 为 `c6741c36`，10-10；`rc.44` 为 `633b4c5b`，10-12；`rc.45` 为 `6cf0d90b`，10-17；`rc.46` 为 `7d8b9126`，10-22；`rc.47` 为 `e835b1bf`，10-24。`rc.40..rc.47` 共 173 个提交，`rc.47..HEAD` 另有 48 个提交）。分支仓库内除默认分支 `clansty` 外，还残留从上游继承的远程分支名（`alpha`、`nightly`、`refactor/shadcn`、`feat/suno` 等）。

### 2. 分叉几何与月度演进节奏

两份代码库的最后共同祖先为 `bee339d279ccecbf8c8a89e14ddbbd902f78bd5d`（2026-04-27 22:07:46 +0800，作者 CaIon，提交说明 `fix: always serialize ratio/price values for all models to ensure fallback during sync delays`）。分支在 2026-04-28 有一条 `Merge tag 'v0.13.2' into clansty`，表明分支所基于的上游底座停留在 `v0.13.2` 时代。

在 `clansty...upstream/main` 的对称差中，分支独有提交为 155 个，上游独有提交为 995 个。分支全历史提交总数为 5826（含共同祖先之前的上游历史）。分支最早的几条独有提交（自 2026-03-27 起）作者日期早于共同祖先 `bee339d`，原因是分支先在自身分支上提交、随后在 4 月底合并过上游标签；其中关于“亲和性包含模型名”（`include_model_name`）的早期提交 SHA 虽不在上游 `main` 历史中，但对应字段在共同祖先 `bee339d` 中已经存在，因此相对共同祖先不构成净新增行为。

按作者日期统计共同祖先之后的月度提交分布如下（分支另有 2026-03 的 2 个早期独有提交未列入下表 4 月之后区间）：

| 月份（2026 年） | 上游 `main` 独有提交（自 04-27 共同祖先起） | `clansty` 分支独有提交 |
|---|---:|---:|
| 3 月 | — | 2 |
| 4 月 | 32（自 04-27 起残月） | 4 |
| 5 月 | 120 | 43 |
| 6 月 | 188 | 25 |
| 7 月 | 179 | 35 |
| 8 月 | 81 | 39 |
| 9 月 | 202 | 4 |
| 10 月 | 183 | 3 |
| 11 月 | 10（截至 11-03 残月） | 0 |
| 合计 | 995 | 155 |

分支的开发集中在 2026 年 5 月至 8 月，9 月与 10 月仅零星维护；上游则在整个半年内保持每月近百至两百个提交的推进速度。

相对共同祖先 `bee339d`，分支的整体代码增删为 403 个文件、`+66137 / -27690` 行；剔除 `web/src/i18n/locales` 翻译文件后为 396 个文件、`+38974 / -2948` 行，净新增文件 177 个（含测试文件 83 个、非测试文件 94 个），删除文件数为 0。

### 3. 产品定位、许可证附加条款与品牌替换

两仓库的许可证正文均为 GNU AGPL-3.0（2007-11-19 版本），但产品叙述与合规声明已明显分化：

- **定位与免责声明**：分支的 `README` 保留了共同祖先时期的标题“新一代大模型网关与AI资产管理系统”（`Next-Generation LLM Gateway and AI Asset Management System`）及“本项目仅供个人学习使用，不保证稳定性，且不提供任何技术支持”免责声明，克隆指令、Docker 镜像（`calciumion/new-api`）、文档地址（`https://docs.newapi.pro`）与商业授权邮箱（`support@quantumnous.com`）均原封未改。上游 `README` 已改为“连接模型、应用与 Agent 的 AI 网关”（`An AI gateway for models, applications, and agents`），法律声明改为面向组织级鉴权、多模型管理、用量分析、成本核算与私有化部署场景，将《生成式人工智能服务管理暂行办法》作为向公众提供服务时的备案、内容安全、实名与日志留存义务说明。
- **AGPLv3 附加条款**：上游新增了共同祖先没有的 `NOTICE` 与 `THIRD-PARTY-LICENSES.md`。`NOTICE` 依据 AGPLv3 第 7 条附加条款，要求修改版在界面 about/legal/footer 中保留句子 `Frontend design and development by New API contributors.` 及指向 `https://github.com/QuantumNous/new-api` 的可见链接，并随二进制、Docker 镜像、前端包与 Electron 安装包分发第三方许可证声明。
- **MuNET 品牌硬编码**：分支通过提交 `10a5038e` 与 `ea430beb` 修改了全部 7 套语言包（`en`、`zh-CN`、`zh-TW`、`ja`、`fr`、`ru`、`vi`），将界面上的 `OIDC` 显示文案统一替换为 `MuNET`，`OIDC ID` 替换为 `MuNET AuID`，“使用 OIDC 继续”替换为“使用 MuNET 账号登录”（`Continue with MuNET account`）。上游界面仍显示为标准 `OIDC`。

---

## 三、代码规模与技术栈对照

### 1. 工作区实测规模

| 指标 | 分支 `clansty` | 上游 `main` |
|---|---:|---:|
| Go 源文件总数 | 690 | 1062 |
| Go 测试文件（`_test.go`） | 119 | 305 |
| Go 非测试文件 | 571 | 757 |
| Go 代码总行数 | 131936 | 268230（非测试 166065，测试 102165） |
| 前端代码文件与行数 | `jsx` 345 个，`tsx` 0 个；`js/jsx` 共 114766 行 | `ts/tsx` 1413 个（`tsx` 918，`ts` 496，`jsx` 0）；共 294031 行 |
| GitHub 语言字节统计 | Go 4049664、JS 3051075、CSS 8245（无 TS） | Go 9586863、TS 2351582、JS 426681、CSS 61517 |
| 独立子模块与目录 | 根目录独有 `AGENTS_QA.md`、`DESIGN.md` | 根目录独有 `.agents`、`assets`、`docker-compose.dev.yml`、`Dockerfile.dev`、`e2e`（1 个文件）、`NOTICE`、`plugins`（29 个路径）、`README.en.md`、`relaykit`（134 个 Go 文件）、`THIRD-PARTY-LICENSES.md`、`tokenkit`（5 个 Go 文件） |

在上游自共同祖先起的 995 个提交中，各目录触及提交数为：`web` 579、`controller` 192、`model` 151、`relaykit` 78、`middleware` 64、`router` 63、`plugins` 38、`pkg/billingexpr` 21、`pkg/jsplugin` 21、`oauth` 7、`service/authz` 6、`tokenkit` 1、`e2e` 1。

### 2. 后端与前端技术栈演进

两边 Go 模块路径仍同为 `github.com/QuantumNous/new-api`，但底层工具链、会话机制和前端框架已经分道扬镳：

| 技术维度 | 分支 `clansty`（延续 `v0.13.2` 底座） | 上游 `main`（`v1.0.0-rc.47` 体系） |
|---|---|---|
| Go 版本 | `go 1.25.1` | 根模块 `go 1.26.10`（`rc.46` 提交说明标注为修复 `CVE-2026-111943` 升级）；`relaykit` 为 `go 1.25.1`；`tokenkit` 为 `go 1.26` |
| 面板会话机制 | 依赖 `gin-contrib/sessions` `v0.0.5` | 提交 `31d70fca`（2026-07-20，`#6329`）移除 `gin-contrib/sessions`；`docs/authentication.md` 写明改为 15 分钟内存 JWT access token + HttpOnly refresh cookie + `user_sessions` 表 |
| 权限与 OIDC 新增库 | 无 Casbin，无 `coreos/go-oidc` | 新增 `casbin/casbin/v2` `v2.135.0`、`coreos/go-oidc/v3` `v3.21.0`（2026-09-06 引入，仅被 `oauth/telegram.go` 用于验证 `https://oauth.telegram.org`，`oauth/oidc.go` 仍为手写 token + userinfo） |
| 高精度十进制库 | `shopspring/decimal` `v1.4.0`（共同祖先已有） | `shopspring/decimal` `v1.4.0`（共同祖先已有） |
| 前端包名与 React 版本 | `react-template`，React `18.2` | `newapi-web` `1.0.0`，React `19.2.7` |
| 构建工具与包管理 | Vite，JavaScript / JSX | rsbuild，TypeScript / TSX，构建与类型检查脚本使用 `bun`，代码检查使用 `oxlint` |
| UI 组件库与样式 | `@douyinfe/semi-ui`（`DESIGN.md` 定义为高密度运维面板）、VChart `1.8` | Tailwind `4.3`、`@base-ui/react` `1.6` |
| 前端路由与状态查询 | `react-router-dom` `6` | `@tanstack/react-router` `1.170`、`@tanstack/react-query` `5.101` |
| 国际化文件 | `en`、`fr`、`ja`、`ru`、`vi`、`zh-CN.json`、`zh-TW.json` | `en`、`fr`、`ja`、`ru`、`vi`、`zh.json`、`zh-TW.json` |

---

## 四、上游在共同祖先之后的演进：新架构与既有遗产的区分

评估上游 `main` 时，需将“2026-04-27 共同祖先中已经存在、仅在后续版本持续打磨或在新版 README 中重点突出的能力”与“共同祖先之后真正新建的模块和客户端面”区分开。

### 1. 共同祖先（2026年 4 月）已存在的能力

以下机制在 `bee339d` 中均已落地，因此分支与上游都具备其基础代码：

- **订阅制模型**：`model/subscription.go` 在共同祖先中已存在。
- **表达式与阶梯计费**：`pkg/billingexpr` 与 `service/tiered_settle.go` 在共同祖先中已存在（此后上游另有 21 个提交继续修改 `billingexpr`）。
- **身份认证扩展**：Passkey（`controller/passkey.go` 与 `go-webauthn`）、2FA（`controller/twofa.go`）、OIDC（`oauth/oidc.go`）在共同祖先中均已存在。
- **多模态与实时路由**：`GET /v1/realtime`、图片、音频、embeddings、rerank 路由，以及 OpenAI Responses、Claude Messages 的 HTTP 路由与 `ChannelTypeCodex = 57` 在共同祖先中均已注册（上游后来将 image generations/edits 主路径移至任务插件宿主协议 `pkg/jsplugin/routing.go`，relay 侧保留回退）。

### 2. 上游在共同祖先之后新建的架构与客户端面

以下目录、模块与路由在共同祖先 `bee339d` 中不存在，属于上游在 995 个提交中新长出的能力：

- **`relaykit/` 独立协议转换模块**：模块路径 `github.com/QuantumNous/new-api/relaykit`，首次出现于提交 `86ac0f77`（2026-07-27，`#6369`），共 78 个提交触及。负责在 OpenAI Chat、OpenAI Responses、Anthropic Messages、Gemini `generateContent` 四种协议之间转换请求、非流式响应与流式事件；HTTP 处理、选渠、鉴权与计费仍留在宿主仓库。模块内定义了 `good`、`fair`、`discouraged` 三档转换质量等级，其中 Claude 与 Gemini 互转因需经由 Chat 协议中转两跳而被标记为 `discouraged`。
- **`tokenkit/` 独立本地分词估算模块**：首次出现于提交 `bb28f2cc`（2026-10-07），仅 1 个提交，要求在 `GOWORK=off` 下仍可独立构建。不请求上游即可在本地估算 token：GPT/o 系列使用 tiktoken，Claude/Gemini 及其他模型使用字符权重，图片按公开尺寸规则计算。
- **`plugins/tasks/` JavaScript 任务插件体系**：首次出现于提交 `eb48396d`（2026-08-29，`#7076`），共 38 个提交触及，配套规范文档为 `docs/plugin-api/v1.md`，用于替换原有的内置任务适配器。已内置 11 个 `meta.key`：`alibaba`、`doubao`、`google`、`hailuo`、`jimeng`、`kling`、`sora`、`sunoapi`、`vertex-ai`、`vidu`、`xai`。前端配套新增了插件市场面板（`web/src/features/task-plugins/components/marketplace-panel.tsx`，对接 `marketplace/sources` 接口）。
- **`service/authz/` 细粒度权限控制**：首次出现于提交 `4aee5f7d`（2026-06-27，`#5755`），共 6 个提交。基于 Casbin `SyncedEnforcer`，按 `sub, obj, act` 三元组鉴权，路由挂载于 `router/api-router.go` 与 `router/authz-router.go`。
- **审计日志**：`model/audit_log.go` 首次出现于提交 `d8cb1774`（2026-09-06），提供 `GET /api/audit` 与 `GET /api/audit/self` 接口。
- **新增客户端协议端点**：
  - `GET /v1/responses` WebSocket 接口：由 `controller.ResponsesWebSocket` 处理，首次出现于 `9fe0457e`（2026-09-14，`#5062`），连接建立后等待 `response.create` 事件。
  - `GET /v1/responses/:response_id`：位于 `pkg/jsplugin/routing.go` 的 `openai_responses.retrieve`。
  - `POST /v1/alpha/search`：格式常量 `openai_alpha_search`，路由注释标明用于 Codex standalone web search。
  - `POST /v1/tools/search` 与 `/v1/tools/fetch`：格式常量 `deepseek_tools`（其适配器内部实现本次调查未展开）。
  - Gemini `countTokens` 显式拦截：上游当前 router 对该路径直接返回 `RelayNotFound`（HTTP 404）；而共同祖先中无此字符串拦截，当时会经由通配路径 `*path` 当作普通 Gemini 请求转发。
- **合成端到端测试 `e2e/`**：仅包含 `e2e/doc_parse_test.go` 1 个文件，在进程内用 Gin + SQLite 测试一个合成的 `doc-parse` 插件，并非针对管理台或真实上游的浏览器测试套件。
- **新增渠道类型 58–63**：依次引入了 Advanced Custom（58）、Sub2API（59）、New API（60）、Task Plugin（61）、vLLM（62）、SGLang（63），详见第六节对照。

---

## 五、`clansty` 分支自有机制详述

分支的 155 个独有提交围绕高密度单方运营场景构建了一套完整改造。以下按运营主题逐项列出经源码核实的机制（对照基准为 2026-11-03 的上游 `main`）。

### 1. 令牌与额度计费

- **令牌子密钥（Subkeys）**：在 `tokens` 表上增加一层父子结构（涉及 `model/token_subkey.go`、`controller/token_subkey.go`、`middleware/auth.go`、`service/billing.go`）。子令牌拥有独立的 `id`、`key`、`name`、`status`，其 `parent_id` 必须指向同一用户名下的根令牌，禁止创建孙级令牌。鉴权时 `ResolveUserToken` 先查出示的子 key 再读取父令牌；`EffectiveToken` 将父令牌的过期时间、剩余额度、无限额度标记、模型限制、IP 白名单、分组、跨组重试、自定义倍率与已用额度复制到请求视图中，仅保留子令牌自身的状态与名称。父令牌被禁用、过期或额度耗尽时，子令牌请求即失败。扣费直接扣减父令牌额度，同时单独累加子令牌的 `used_quota`（若子令牌写入失败则回滚）。创建子 key 计入用户的令牌数量上限；删除父令牌时级联删除子令牌。用户侧接口为 `GET/POST /api/token/:id/subkeys`、`DELETE` 子 key 以及父 key 视角的 `/api/token/subkeys`（列表接口对 key 打码，单独接口返回带 `sk-` 前缀的完整 key）；管理员亦可在用户管理页操作子 key。上游无 `ParentId` 及子密钥机制。
- **用户透支（Overdraft）**：`users.allow_overdraft` 默认 `false`。开启后，钱包预扣与正向结算跳过剩余额度检查，直接调用 `DecreaseUserQuota`，允许 `users.quota` 扣成负数；音频额度检查与 Midjourney 处理路径同样接入了该开关，但订阅资金路径不受影响。上游无此字段。
- **令牌自定义额度倍率（Custom Ratio）**：`tokens.custom_ratio` 要求大于等于 0，值为 0 时代表倍率 1。当 `custom_ratio > 0` 且分组倍率大于 0 时，计算 `TokenQuotaRatio = customRatio / actualGroupRatio`；令牌自身扣减 `round(userQuota * TokenQuotaRatio)`，而用户钱包依然扣减 `userQuota`。子 key 继承父令牌的自定义倍率。上游无此字段语义。
- **模型忽略分组特殊倍率**：开启 `ModelIgnoreGroupSpecialRatio` 后，`HandleGroupRatio` 仅使用基础分组倍率 `GetGroupRatio(usingGroup)`，不再套用用户组与使用组之间的交叉特殊倍率 `GetGroupGroupRatio(userGroup, usingGroup)`。上游无此开关。
- **兑换码修改用户组**：`redemptions.group` 字段为空时仅增加额度；非空时在同一数据库事务中既增加额度又修改 `users.group`，随后刷新分组缓存（充值日志仅记录额度与兑换码 id，不记录新分组名）。上游兑换码无 `Group` 字段。
- **模型用途限制（ModelPurpose）**：允许的用途枚举为 `chat`、`image`、`embedding`、`audio`、`rerank`、`moderation`、`video`、`music`。服务启动时会种子化一份默认映射表（涵盖 `gpt-image`、`dall-e`、`mj_*`、`embeddings`、`whisper/tts`、`sora/veo`、`suno` 等），管理员保存设置时会整表替换。分发器在选渠之前将请求路由映射为用途：未在表中配置条目的模型允许所有用途；已配置条目的模型若请求用途不在允许列表中，直接返回 HTTP 400。上游无此机制。
- **按令牌的阶梯价目查询接口**：阶梯表达式 `tiered_expr` 结算在共同祖先中已存在，分支新增了 `controller/token_pricing.go`（`GET/POST /api/pricing/token`），按出示的令牌返回各档位的输入、输出、缓存价格，以及叠加了令牌自定义倍率与“忽略分组特殊倍率”之后的最终乘数。控制器会解析 `tier(` 调用并保留条件文本，对三段式 `len` 比较条件合成互补运算符。上游虽结算 `tiered_expr`，但无此接口。
- **上游成本日志（Upstream Cost）**：`logs.cost` 为可空列。消费结算时，按应用分组倍率之前的原始额度乘以渠道的有效上游倍率冻结成本（自动获取的 `upstream_rate_multiplier` 优先于手动配置值；若无倍率则 `cost` 保持 `null`）。非管理员查询日志时 `Cost` 字段被置为 `nil`。文本、音频、阶梯计费、异步任务、Midjourney、违规扣费与渠道测试路径均已接入。上游无 `logs.cost` 字段。
- **Claude 缓存读取按缓存创建计费**：渠道级开关 `claude_cache_read_as_cache_creation`（界面文案为“不要缓存”）。开启后，在向下游返回用量并执行计费之前，将 `cache_read_input_tokens` 累加进 `cache_creation_input_tokens`（若存在 `ephemeral_5m` 对象也一并累加），随后将 `cache_read_input_tokens` 归零。上游无此开关。
- **图像响应空输出拦截**：在 `relay/channel/openai/relay_image.go`（提交 `d292f5e8`）中，若 OpenAI 图像接口返回 HTTP 200 但携带 `error.type`（例如审核拦截映射为 400），或 JSON `data` 为空、条目内不含图片数据，则判定为 HTTP 502，不向客户端拷贝响应体且不按成功结算（Responses 风格且携带 `result` 的 `image_generation_call` 视为成功）。若流式响应已向客户端转发过事件但结束时仍无图片，则返回 502 并标记 `skip-retry`，不再触发故障转移。上游是否有完全等价的空图拦截路径未做逐行比对，保留为不确定。

### 2. 调度、亲和性与故障恢复

- **渠道组与并行竞速（Channel Group Racing）**：渠道可标记 `is_group = true`，其成员存储在独立的 `channel_members` 表中（含名称、key、可选 base URL、可选代理、状态、测试时间、响应时间、余额、禁用原因；转为渠道组时原有的多 key `ChannelInfo` 会被清空）。`parallel_requests` 限制在 1 至 4 之间。调度时若命中亲和成员则将其排在首位，通过 `errgroup` 并发请求，每个成员持有一份 Gin 上下文拷贝与 `RelayInfo.CloneForAttempt`。若配置了 `affinity_parallel_delay`（默认 3000ms，UI 下限 100ms），则先单独请求优先成员，待其失败或延迟到期后再放行其余并发成员。率先写出响应体的成员获胜，其余成员通过 context 取消；若出现两次真实失败且尚无赢家，则再等待 2 秒后取消其余尝试；若遇到 `skip-retry` 错误且尚无赢家则立即中止其余尝试。额度预扣在竞速开始前执行一次，`CloneForAttempt` 浅拷贝 `Billing`，`Settle` 在互斥锁保护下仅成功执行一次。竞速适用的请求模式限定为：chat、completions、responses、responses compact、Claude 以及 Gemini 的 `GeminiChatRequest`。成员可被单独自动禁用；若组内无任何启用成员，整个渠道组会被自动禁用并关闭 `ability`；当有成员恢复且组处于自动禁用状态时，组会自动重新启用。上游无 `channel_members` 表及并行竞速机制（仅保留 `channels.key` + `ChannelInfo` JSON 的 random/polling 多 key 模式，分支在渠道组之外也保留了该传统模式）。
- **亲和性增量改造**：
  - `using_group_include`：为空时对所有分组生效，非空时要求当前使用分组精确匹配。上游亲和规则无此字段。
  - 请求头来源标识：分支使用字符串 `"header"`，上游使用 `"request_header"`，双方持久化的规则 JSON 不可互换。
  - `inject_affinity_user_id`：开启后按 `user_{64位hex设备}_{账号}_{会话uuid}` 格式生成标识（设备段为 `SHA-256("affinity-device:" + userId)`，账号段为空，会话段为亲和 cache key 后缀的 UUIDv5），注入到 Anthropic 请求的 `metadata.user_id` 中，覆盖 Claude、chat 与 responses 路径；若生成为空则作为 `skip-retry` 转换错误中止。上游无此注入开关。
  - 管理员强制亲和：提供 `POST/DELETE /api/channel/:id/affinity_force` 接口，仅对启用中的渠道生效，按该渠道所属分组及匹配规则的模型写入混合缓存 `new-api:channel_affinity_force:v1`（有 Redis 用 Redis，否则存内存，不落 SQL），固定有效期 5 分钟；生效期间解析亲和性直接返回该渠道 id，禁用渠道时自动清除其强制记录。上游无此功能。
  - 原地重试（`in_place_retry_times`）：渠道级配置，默认 0。当失败渠道恰好是亲和渠道、重试次数未达上限、`shouldRetry` 为真，且不属于“命中自动禁用条件且开启 `auto_ban`”时，直接对同一渠道再次重试（覆盖普通转发与任务转发）。上游采用的则是亲和重试偏移、`session_mode`（`inherit`/`prefer`/`strict`）与 `keep_on_channel_disabled`，双方实现路径不同。
- **按模型的重试策略与跳过重试关键词**：`model_retry_setting.models[model]` 中的 `retry_times` 可按模型覆盖全局重试次数；`priority_strategy` 为空或 `same` 时先耗尽同优先级渠道，为 `next` 时单次失败即跳往下一优先级。另设 `AutomaticSkipRetryKeywords`（换行分隔列表，使用大小写不敏感的 Aho-Corasick 自动机匹配）：一旦上游错误信息命中关键词即停止后续重试。该列表与命中后禁用渠道的 `AutomaticDisableKeywords` 相互独立。上游仅有全局重试次数、渠道优先级与自动禁用关键词，无按模型重试表及跳过重试关键词。
- **首响应超时（First Response Timeout）**：`channels.response_timeout`（单位秒，0 或空表示关闭），仅作用于流式 chat、responses 与 Claude 请求，不用于渠道测试；当请求携带 web search 或内置工具（`code_interpreter`、`computer`、`computer_use_preview`、`file_search`、`image_generation`、`mcp`、`web_search`、`web_search_preview`）时自动豁免。计时器在读到首个响应体字节时停止，若超时则取消上游 context，且该超时错误不会触发 `ShouldDisableChannel` 自动禁用渠道。上游仅有 TTFT 指标统计展示，不通过该字段主动取消上游请求。
- **渠道级自动禁用恢复开关**：`channels.auto_recover`（空值视为开启，默认 1），支持批量接口 `POST /api/channel/batch/auto_recover`。定时测试始终跳过手动禁用的渠道；对于自动禁用的渠道，仅当 `GetAutoRecover()` 为真时才纳入测试恢复循环。上游则是通过全局 `monitor_setting.channel_test_mode`（`scheduled_all`、`auto_ban_only`、`passive_recovery`）控制，无逐渠道列。

### 3. 渠道种类与余额探测扩展

- **OpenCodeGo（渠道类型 59）**：内嵌 OpenAI 适配器，在 `adaptor.go` 中维护静态模型名单（涵盖 `glm`、`kimi`、`deepseek`、`mimo`、`minimax`、`qwen`）。其中 `minimax-*` 与 `qwen*` 路由至 `/v1/messages` 并使用 Claude 适配器，其余模型路由至 `/v1/chat/completions`；Gemini 入站请求会先转换为 OpenAI 格式再进入该分支开关。上游无此适配器，且 59 号被用于 `Sub2API`。
- **PassThrough（渠道类型 58）**：针对 Claude、Gemini、OpenAI、Responses、compaction、alpha search 原样转发请求路径（当 Gemini 源模型与映射后的上游模型名称不同时，会改写路径中的模型段），剔除 URL query 中的 `key`、`api_key`、`access_token`，按协议分别注入 `x-api-key`、`x-goog-api-key` 或 `Bearer` 鉴权头，转换函数直接原样返回请求体。上游的 58 号是 `Advanced Custom` 路由表，并非此适配器。
- **AdvancedPassThrough（渠道类型 60）**：要求同时配置 `AdvancedOpenAIBaseURL` 与 `AdvancedAnthropicBaseURL`，根据最终目标格式决定选用哪个 Base URL：协议收敛为 `/v1/messages` 或 `/v1/chat/completions`，其他格式保留剔除凭据 query 后的原始路径；Gemini 入站先转为 OpenAI 格式；alpha search 发往 OpenAI Base URL 的 `/v1/alpha/search`。上游无此适配器，60 号为 `New API` 渠道。
- **sub2api 余额查询模式**：在分支中，`sub2api` 是可挂载在任意渠道上的余额探测与倍率同步模式，而非渠道类型。渠道表新增了用户名、密码（JSON 序列化不输出）、access/refresh token、过期时间、三个倍率字段及组名/组描述。探测时向 `{root}/api/v1/auth/login` 发送带 `not_in_cn_confirmed: true` 的登录请求（若返回 `requires_2fa` 则立即报错停止），或向 `/api/v1/auth/refresh` 刷新令牌。独立表 `sub2_api_auth_credentials` 以规范化后的 `base URL + 用户名 + 密码` 的 SHA-256 作为主键存储凭据（启动迁移会将渠道上已有的最新 token 对拷入该表并清空渠道表上的 token 列），配合进程内缓存与 `singleflight` 复用会话。倍率从 keys 查询、`groups/rates` 与 `/v1/sub2api/billing` 获取，渠道上声明的倍率优先于登录拉取的倍率，并用于写入日志成本。密码在数据库中以明文 `text` 列存储，源码中未见加密包装。
- **hyl2api 余额查询模式**：向 `{root}/user/api/quota` 发起 GET 请求，要求响应中包含 `usd.remaining` 字段。上游无此模式。

### 4. 管理台与可观测性

- **性能分位页面**：管理员接口 `GET /api/log/performance`（时间跨度最长 31 天），支持按模型或渠道分组，读取消费日志中的 `other.frt` 计算 TTFT 与 TPS，并在进程内按最近秩（nearest-rank）计算 `p50`、`p95`、`p99`（独立于旧有的 `/api/performance/stats`）。上游使用 `pkg/perf_metrics` 在 Redis 中累加 TTFT，无此分位统计接口。
- **提示缓存命中率**：从消费日志 `other` 中提取 `cache read` 与 `cache write`（Anthropic 语义下分母同时加上读写 token），按 `read / 分母` 计算命中比例（分母小于等于 0 时为空）。上游维护设置中的 `cache hits` 统计的是本地磁盘/内存对象缓存命中次数，并非大模型提示词缓存比例。
- **桑基流量图（Sankey Flow）**：分支提供独立页面（默认展示近 7 天），调用 `GET /api/data/flow` 与 `/flow/self`，数据源自 `quota_data`，管理员视图路径为 `user → group → model → channel`，普通用户视图为 `token → group → model`，并支持带 `node` 参数的根视图，可切换额度、token、次数三项指标。该能力两边都有：上游的 `model/usedata_flow.go` 与分支仅相差一条注释，阶段划分完全一致；上游将其集成在仪表盘内，并额外实现了遮罩、高亮、溢出“其他”节点聚合以及单元测试（双方谁先编写未做历史溯源）。
- **在途请求计数（Inflight Count）**：在进程内存中维护映射表（不落数据库），在用户列表和渠道列表接口中返回 `inflight_count`（渠道组竞速的每次并发尝试均会使组渠道 id 的在途计数加一）。由于在途数不在 SQL 表中，按 `inflight` 排序无法下推至数据库，`GetUsers` 会先拉取全量过滤结果集再在内存排序。上游无此统计。
- **渠道折叠与批量改模型**：`channels.collapsed` 仅影响管理端列表展示（拆分为 `items` 与 `collapsed_items` 并支持批量接口），不改变路由逻辑；`POST /api/channel/batch/models` 支持在单个事务内批量增删多个渠道的模型并同步更新 `ability`。上游无此两项接口。
- **按渠道操练场**：管理员可通过 `GET /api/channel/playground` 列出包含禁用状态在内的所有渠道，并通过 `GET /api/user/models?channel_id=` 获取指定渠道的模型列表（非管理员请求禁用渠道会被拒绝）。上游操练场仅按分组加载模型。
- **用户置顶、令牌代管与 OIDC 头像**：`users.pinned_time` 非 0 时在用户列表中置顶排序（删除用户时清零）；管理员可在用户列表页直接查看、批量删除该用户的令牌与子 key 并查看明文 key（`AdminGetUserTokens`）；支持配置 `picture_endpoint` 模板，保存并展示 OIDC `userinfo` 返回的 `picture` 头像（无图时回退为首字母头像），登录信息附带 oauth 绑定与 `oidc_id`。上游无置顶字段、无 `AdminGetUserTokens`、无 `picture_endpoint`（仅展示 `oidc_id` 与生成色块头像）。

### 5. 协议转换与 Claude 日期清洗

- **手写协议互转**：分支无 `relaykit` 目录，直接在 `relay` 层内手写了 Responses 转 Claude、Claude 转 Responses 流、Claude 请求转 Responses 请求，以及渠道级开关 `responses_via_chat_completions`（将 Responses 请求转为 Chat Completions 发往上游，再将响应映射回 Responses 事件；提交 `97a9e321` 注明用于对齐 `cc-switch` 的互转缺陷）。上游统一由 `relaykit` 的 `oai_responses` 与 `claude_messages` 转换器及 golden 测试文件承担。
- **`system_clean.go` 日期格式规范化**：仅作用于 Claude 处理路径（调用点位于 `relay/claude_handler.go` 与 `relay/channel/claude/relay-claude.go`）。其逻辑是将提示词中 `"Today's date"` 短语里的三种弯撇号统一规范化为 ASCII 直撇号，并将该短语紧随其后的斜杠分隔日期改写为短横线分隔日期；对于系统提示词（system prompt），还会额外把文本中恰好等于服务器本地时区“今天、昨天、明天”的斜杠日期改写为短横线格式；对于用户消息，仅处理第一条用户消息，且不做“今天±1天”的额外日期替换，后续消息保持原样；原始 body 透传路径仅改写文本字段。该文件属于日期格式规范化处理，并非安全策略剥离。上游无此处理。

### 6. 优雅关闭、限流豁免与压缩中间件

- **优雅关闭与 503 摘流**：收到 `SIGINT`/`SIGTERM` 后，分支先置位原子标记并取消 `ShutdownCtx`，此后健康检查接口 `/api/status` 立即返回 HTTP 503，供 Swarm 或负载均衡器摘除流量；接着默认睡眠排空 5 秒（可通过 `SHUTDOWN_DRAIN_DELAY_SECONDS` 配置），随后调用 HTTP `Shutdown`（默认超时 14 分钟，超时后强制 `Close` 并预留 30 秒）；再由 `WaitBackground` 最多等待 60 秒以收拢 `GoTracked` 追踪的后台协程，之后停止批量更新、执行最多 30 秒的刷新重试循环，最后依次关闭 Redis 与数据库连接。上游 `main.go` 收到信号后直接执行默认 120 秒（`SHUTDOWN_TIMEOUT_SECONDS`）的 HTTP `Shutdown`，并在开启数据导出时调用 `SaveQuotaDataCache`，无 `/api/status` 503 翻转、无排空延迟、无后台协程等待组及刷新重试循环。
- **令牌与用量接口限流豁免**：提交 `71c26f6e` 的提交说明写为 `remove ratelimit`。实际代码保留了包裹其余 API 的 `GlobalAPIRateLimit`，仅将 `POST`/`GET` 的 `/api/usage/token`、`/api/pricing/token`、`/api/token/subkeys`、`/api/log/token` 以及路径前缀 `/api/token/subkeys/` 在进入全局限流器之前提前放行返回；同时移除了 `/api/usage` 路由组上的 `CriticalRateLimit`，仅保留 CORS 中间件（测试文件验证了当限额设为 1 时，上述路径的第二次请求依然通过，而 `/api/status` 第二次请求返回 429）。上游无此豁免名单。
- **请求体压缩解码**：唯一一条非 Clansty 本人撰写的独有提交来自 `akachan`（`5c535cbd`，2026-05-18），在中间件中自动解压 `gzip`/`zstd` 请求体，遇到未知或非法编码时返回 HTTP 415。

### 7. 只见于提交说明的主题

以下改动出现在分支的 git 提交说明中，但本次调查未逐文件打开核对具体实现细节，仅作为提交记录列出，不作为已核实机制：隐藏成功重试后的失败日志；日志记录 `User-Agent`；PostgreSQL 日志分组筛选修复；空日志分页返回空数组；钱包与用户搜索支持按备注检索；首页在导航模块关闭时自动重定向至控制台或登录页；密码注册关闭时隐藏注册按钮；公告仅对登录用户可见；移除密码最大长度与控制台内容长度限制；渠道优先级排序稳定性修复；在同优先级同伴尝试之后再重试失败渠道；仪表盘日期浏览与管理员成本分布展示；性能图表时间轴回绕修复。

---

## 六、同名异物与编号冲突专节

在两个分支之间迁移配置、复用数据库或讨论功能名称时，有四处“名称或编号相同、但底层语义完全不同”的设计，直接混用会导致静默路由错乱或计费异常。

### 1. 渠道类型号（Channel Type ID）58–64 冲突表

渠道类型 `0` 至 `57` 的常量编号在两边保持一致（包括历史留空的 `29`、`30`、`32`、`39`；仅个别显示名有差异，例如 57 号在上游显示为 `ChatGPT Subscription (Codex)`，以及 `Doubao` 与 `DoubaoVideo` 的命名区分）。但从 `58` 号开始，两边各自独立递增了枚举值与哨兵常量 `ChannelTypeDummy`：

| 类型号 (ID) | `clansty` 分支定义 | 上游 `main` 定义 | 冲突后果 |
|---:|---|---|---|
| `58` | `PassThrough`（原样路径透传适配器） | `Advanced Custom`（按路由配置转换器与鉴权方式的自定义渠道） | 数据库互指时，58 号渠道会被对方按完全不同的适配器逻辑解析转发 |
| `59` | `OpenCode Go`（按模型名前缀分流 OpenAI/Claude 的静态名单适配器） | `Sub2API`（转发至 Sub2API 网关的 relay 适配器） | 分支的 OpenCode Go 渠道在上游会被当成 Sub2API 网关；上游的 Sub2API 渠道在分支会被当成 OpenCode Go |
| `60` | `AdvancedPassThrough`（双 Base URL 自动分流 OpenAI/Anthropic 的透传适配器） | `New API`（网关渠道，不是双 Base URL 透传） | 60 号渠道在两边分别指向双地址透传与 New API 网关适配器 |
| `61` | `ChannelTypeDummy`（计数哨兵，无对应渠道） | `Task Plugin`（JS 任务插件宿主渠道） | 分支无法识别上游的 61 号任务插件渠道 |
| `62` | 无定义（越界） | `vLLM`（`IsAdvancedCustomChannel` 对 58、62、63 为真） | 分支无法识别 |
| `63` | 无定义（越界） | `SGLang` | 分支无法识别 |
| `64` | 无定义（越界） | `ChannelTypeDummy`（上游计数哨兵） | — |

### 2. `sub2api`：余额探测模式与转发渠道适配器

- 在 `clansty` 分支中，`sub2api` 是任何渠道都可开启的**余额查询与倍率同步模式**，通过账号密码登录目标站点的 `/api/v1/auth/login` 与 `/api/v1/auth/refresh`，把凭据存入 `sub2_api_auth_credentials` 表，并拉取倍率计算上游成本。
- 在上游 `main` 中，`Sub2API`（类型号 `59`，提交 `2d23cdf2`，2026-07-26）是一个**渠道转发适配器**，内嵌 `newapi.Adaptor`，静态模型列表为空，不包含账号密码登录、不含密码列、无共享凭据表、也无上游成本倍率列。

### 3. “透传”与自定义渠道：`PassThrough` / `AdvancedPassThrough` 与 `Advanced Custom` / `New API`

- 分支的 `PassThrough`（58）与 `AdvancedPassThrough`（60）旨在绕过网关标准协议重编码，将原始请求路径与请求体直接送往上游（60 号按最终格式在 OpenAI Base URL 与 Anthropic Base URL 之间二选一）。
- 上游的 `Advanced Custom`（58，提交 `3f2c0aed`，2026-06-18，`#5590`）是一套按路由配置协议转换器、配置鉴权位置（`none`/`header`/`query`）并可按需勾选 `pass_through_body` 的自定义路由表；上游的 `New API`（60，提交 `398cdafe`，2026-07-27）则是网关渠道，不是这套双地址透传。

### 4. 协议转换与亲和性请求头标识

- **协议转换**：两边都支持 OpenAI Responses 与 Anthropic Claude Messages 互转，但分支是直接写在 `relay` 层内的手写转换逻辑加 `responses_via_chat_completions` 开关；上游是抽离为独立 Go 模块的 `relaykit` 转换器矩阵。
- **亲和性规则 JSON**：从请求头提取亲和键时，分支规则存为 `"header"`，上游存为 `"request_header"`，直接迁移配置表会导致亲和性匹配静默失效。

---

## 七、合并、升级或数据库对调的工程风险与不确定项

### 1. 数据库表结构（GORM AutoMigrate）不可逆污染

分支启动时的 `AutoMigrate` 会自动创建两张新表并向五张核心表追加列：

- **新建表**：`channel_members`、`sub2_api_auth_credentials`。
- **追加列**：`channels`（渠道组、竞速、首响应超时、自动恢复、折叠、sub2api、双透传 URL、Claude 缓存计费等列）、`tokens`（`parent_id`、`custom_ratio`）、`users`（`allow_overdraft`、`pinned_time`）、`redemptions`（`group`）、`logs`（`cost`、`token_root_id`、`user_agent`）。

若将分支编译出的二进制文件指向正在运行的上游数据库，启动迁移会立即改动上游表结构；反之，若将上游二进制指向分支数据库，上游不仅忽略上述新增列（导致子密钥层级、透支开关、自定义倍率全部失效），还会将 `channels.type` 为 `58`、`59`、`60` 的存量渠道按上游语义错误路由，并因缺少 `channel_members` 读取逻辑而使所有渠道组因 `channels.key` 为空而无法工作。

### 2. 已核实的 11 项运营与迁移风险

1. **类型号 58–60 静默改道**：混用数据库或导入导出渠道配置时，58、59、60 号渠道在启动与转发时均不报错，会直接按另一套适配器逻辑处理；上游的 61–63 号渠道在分支中无法表达。
2. **前后端与核心转发层断代**：两边在共同祖先之后各自演进了 155 与 995 个提交。前端已是两套完全不同的工程（Semi UI + React 18 + Vite + JSX 对比 Tailwind 4 + React 19 + rsbuild + TanStack Router/Query + TSX），后端协议转换、任务插件（`plugins/tasks/`）、权限（`service/authz`）与会话（Gin session 对比 JWT）均不兼容。将分支合并到当前上游在工程量上等同于在 `v1.0.0-rc.47` 上重新实现一遍分支特性，无法通过常规 `git rebase` 完成。
3. **许可证与品牌归属混杂**：两边均为 AGPL-3.0，上游 `NOTICE` 要求修改版保留贡献者声明与上游链接；而分支 `README` 仍指向克隆上游仓库并联系 QuantumNous 商业授权邮箱，同时全部语言包又被硬编码为 `MuNET` 品牌，直接分发或回拷 locale JSON 会将 `MuNET` 品牌带入非 MuNET 站点。
4. **计费语义多处改变默认预期**：
   - `allow_overdraft` 允许用户钱包额度扣为负数；
   - `custom_ratio` 使令牌扣减额度与用户钱包扣减额度脱钩；
   - `ModelIgnoreGroupSpecialRatio` 会少乘一层组间特殊倍率；
   - `claude_cache_read_as_cache_creation` 将缓存命中读取的 token 按缓存写入价格计费；
   - 无图片输出的 HTTP 200 图像响应改为返回 502 且不结算；
   - 带 `group` 的兑换码在充值同时直接改写用户所属分组；
   - `ModelPurpose` 会对种子列表中模型的跨用途路由直接返回 400；
   - `AutomaticSkipRetryKeywords` 命中后会提前终止故障转移。
5. **渠道组竞速放大上游调用成本**：单次客户端请求最多并发打向 4 个上游成员。虽然本地钱包仅预扣一次，但取消信号是在首个赢家写出响应体之后才发出的，此时输家请求往往已经发往上游并可能已在上游产生计费。
6. **Claude 提示词被静默改写**：`system_clean.go` 会修改客户端发来的 Claude 系统提示词与首条用户消息中的 `"Today's date"` 撇号和日期格式，还会将系统提示词中恰好等于服务器本地“今天、昨天、明天”的斜杠日期替换为短横线。
7. **令牌与用量接口脱离全局限流**：`/api/usage/token`、`/api/pricing/token`、`/api/token/subkeys`、`/api/log/token` 及 `/api/token/subkeys/` 前缀在 `GlobalAPIRateLimit` 之前返回，且 `/api/usage` 移除了 `CriticalRateLimit`。这些路径是否仍要登录，以各自的鉴权中间件为准；这里核实的是它们不再经过全局限流。
8. **`sub2api` 明文密码与共享会话耦合**：为探测余额，第三方站点的登录密码以明文 `text` 列存储在数据库中，且相同 `base URL + 用户名 + 密码` 的渠道共享同一行 `sub2_api_auth_credentials` 记录，任一渠道触发刷新都会改变同凭据其他渠道读到的 token。
9. **渠道组成员故障级联禁用整组**：当组内启用成员全部被自动禁用时，整个渠道组会被自动禁用并关闭 `ability`。
10. **首响应超时与自动恢复的循环行为**：`response_timeout` 超时被设计为不触发自动禁用渠道；而 `auto_recover` 默认开启（值为 1），意味着被其他错误自动禁用的渠道会自动回到定时测试恢复循环中，除非管理员显式关闭。
11. **会话升级断层**：从分支（或共同祖先）切往上游时，原有的 Gin session 登录态全部失效，客户端与反向代理需适配上游的内存 JWT + HttpOnly refresh cookie 机制。

### 3. 调查保留的不确定项

以下问题在静态代码核查中未做完整穿透，决策时应视为未验证项：

- **竞速结算的并发竞态细节**：渠道组竞速中多个成员共享同一个 `BillingSession`，虽然 `Settle` 受互斥锁保护仅首次写入生效，但失败尝试是否有可能在赢家之前抢先触发 `Settle` 调用链，本次调查未将全部调用顺序追踪完毕。
- **上游图像空输出处理**：未完整比对上游 `main` 的图像处理路径是否已包含与分支 `d292f5e8` 等价的空图片输出拦截。
- **桑基图代码起源**：两边 `model/usedata_flow.go` 几乎完全相同（分支上有提交 `fa620f87`），未追溯最初作者归属。
- **`ModelPurpose` 空配置重启行为**：种子化默认表之后，若管理员将选项保存为空，后续重启时的覆盖行为未完全确认。
- **渠道类型 0–57 的适配器实现细节**：仅核对了常量编号块的一致性，未对 0 至 57 号的每个适配器做逐文件 diff，且分支 HEAD `d292f5e8` 是否私下挑拣移植过个别上游修复未逐行证明。
- **DeepSeek tools 适配器内部实现**：上游新增的 `POST /v1/tools/search` 与 `/v1/tools/fetch`（`deepseek_tools`）未打开适配器内部代码。
- **运行时表现**：未执行测试套件，未验证实际流量下的并发稳定性。

---

## 八、决策结论

- **对于只需要当前主流 AI 网关能力的团队**：应直接部署上游 `QuantumNous/new-api`（`main` / `v1.0.0-rc.47`），以获得 `relaykit` 四协议转换、`tokenkit` 本地分词、JS 任务插件、Casbin 细粒度权限、审计日志、Responses WebSocket 以及 React 19 + TypeScript 前端，避免停留在 2026 年 4 月的 `v0.13.2` 底座上。
- **对于已经在运行 `clansty` 分支（或依赖 MuNET 式运营特性）的团队**：应将其视为一套锁定在 `v0.13.2` 底座上的专有定制分支独立维护，切勿直接将二进制与上游对调或共用数据库，并在日常运维中重点管控 `58–60` 号渠道定义、渠道组竞速带来的多倍上游调用、`sub2api` 明文密码存储以及令牌查询接口的限流豁免。
- **对于计划将分支特性迁回上游的开发者**：不存在直接 `git merge` 或 `git rebase` 的路径，唯一可行方案是以当前上游 `main` 为基座，将所需能力（如令牌子密钥、渠道组与竞速、上游成本列或首响应超时）按上游现有的 `relaykit`、JWT 会话、避开 `58–63` 号的渠道枚举以及 React 19 + TSX 前端架构逐项重新设计并独立移植。
