# nam 项目文档索引与开工指引

nam（newapi-manager）是给 clansty 版 NewAPI 用的上游管理工具。**M0 实测、M1 骨架、M1.5 嵌入、M2 上游与渠道已完成**，下一步是 M3（价格与余额）。<u>代码结构、运行和测试方法见 `docs/development.md`</u>。

## 文档

### 先读哪些

- **`docs/requirements.md`：需求，冲突时以它为准**  
  §3 是每一轮对话的原文和结论，§4 是当前有效需求（带编号和状态），§6 是待确认问题。
- **`.omo/plans/newapi-manager-plan.md`：实施计划**  
  架构、数据表、适配器、统计、定价、里程碑和每个里程碑的 QA 场景。第 16 轮的版本通过了 Momus 审查（OKAY）；第 18 轮改动后主人决定<u>不再重新审查</u>，直接开工。
- **`docs/feature-split.md`：功能划分**  
  哪些改 NewAPI 代码、哪些只改配置、哪些放在 nam 核心、两个适配器各做什么。
- **`docs/ui-features.md`：界面功能说明（<u>功能规格</u>）**  
  每个页面的表格列、按钮、弹窗和用户逻辑。界面从零重新实现，以它为准；视觉设计不在其中。
- **`docs/ui/mockup.html`、`docs/ui/sitemap.svg`：已过时**  
  只作历史留存，<u>不作为实现依据</u>，也不要参考其代码和样式。
- **`docs/research/new-api-fork-report.md`：clansty 分支调研**  
  clansty 与官方 new-api 的差异、同步状态和自有功能。

### 不在 git 里的参考资料

- **`temp/clansty-new-api`**：clansty 分支源码（HEAD d292f5e8），补丁和嵌入适配器都基于它。
- **`temp/metapi`**：调研过的替代方案源码，只作参考。
- **`temp/ui-src`**：旧界面草稿，已过时，不要参考。
- **`F:\Mess\oc-modal-configurator`**：表格框选、复制粘贴的**纯逻辑**可以复用（`selection/machine.ts`、`useSpreadsheetSelection.ts`），它的界面不能复用。

## 已经定下来的架构

### 运行方式

- **nam 核心 + 两个适配器**  
  核心不引用 new-api 的任何包，通过 `ChannelStore`、`OptionStore`、`LogSource`、`Auth` 四个接口访问 NewAPI。
- **嵌入模式（生产默认）**  
  编译进 NewAPI 进程，直接调用 NewAPI 的 model 和 controller。NewAPI 只改启动一行、路由注册、`go.mod` 和 Dockerfile。
- **独立模式（开发和退路）**  
  单独运行，通过 NewAPI 管理 API 读写，自带登录。<u>计划要求先做独立适配器</u>（M1），再做嵌入适配器（M1.5）。

### 关键约束

- **只给 root 用**：嵌入模式所有接口套 `RootAuth()`，高危操作再加二次验证；独立模式要求填 root 的访问令牌。
- **数据单独存放**：nam 的表全部在 `nam.db`，不放进 NewAPI 的数据库。
- **渠道以 NewAPI 为准**：nam 不复制渠道字段，只按 `channel_id` 存扩展信息。
- **不能拖垮网关**：nam 的所有后台 goroutine 都要经过统一的 recover 包装；写渠道和配置必须走 NewAPI 的完整流程（abilities、渠道缓存、价格缓存）。
- **前端独立**：React + TypeScript，挂在 `/nam/` 下，不改 NewAPI 现有前端，不用 iframe。接口统一用 `/api/nam/` 前缀。界面从零重新实现，只依据 `ui-features.md`。

### 定价方式

- **厂商分组 + auto**
    - 每个厂商对应 NewAPI 的一个分组，该厂商所有渠道都放进这个分组；用户令牌用 `auto`。
    - NewAPI 里的模型价格保持**原厂价**，nam 不自动改。
    - 每个厂商设一个**售价**（元 / 原厂 1 美元），nam <u>只写分组倍率</u>：`GroupRatio` = 售价 ÷ 站点汇率，另外维护 `AutoGroups`、`UserUsableGroups`。
    - 模型价格和原厂价相差超过 5% 只提示，不自动写。
    - 用户分组加价以后再做。

## 新会话怎么开工

### 下一步

- **Go 1.25.14 已装在 `D:\Go`**，用户环境变量已设好；下载依赖要走代理 `127.0.0.1:7890`。
- **M0 已完成**：结论在 `docs/m0-findings.md`。第 ⑦ 项（sub2api）<u>还需要主人提供账号</u>；**P2 不需要**；Gemini 走 type=58 计费有问题，待定。
- **M1 / M1.5 / M2 已完成**：详见 `docs/development.md` 的"当前进度"。
- **第 19 轮修订**已写进需求和计划：Gemini 只建原生渠道（type=24）、`UserUsableGroups` 必须含 `auto` 等。
- **下一步是 M3**。

### 给新 Agent 的提醒

- **先读本文件和 `requirements.md` 的"参数说明"**，再读计划。
- **文档和界面里不要出现字母参数**（k、r、m、S 之类），一律用中文名。
- **界面从零重新实现**：不要打开 `docs/ui/mockup.html` 或 `temp/ui-src` 找样式和代码。
- **定价只写分组倍率**：nam 不自动改 NewAPI 的模型价格。
- **编码任务按主人的模型分工交给子 Agent**；子 Agent 的提示要短、要具体，<u>让它直接动手，不要长时间思考</u>，否则容易中断。

### 还没定的事

- **要另外讨论的**  
  优先级算法和故障转移（需求 F3、计划 §9），本期只留接口。

### 环境约定

- 临时文件放在 `temp/`；用到 Python 时在项目里建 venv。
- **Windows 写代码、跑测试**  
  Go 1.25.x 在 `D:\Go`，GOPATH、模块缓存、构建缓存都在这个目录下；Node 用现有的 v24。
- **WSL 只跑 docker**（29.1.3）  
  运行 NewAPI、M0 实测、QA 环境和镜像构建。<u>WSL 里不装 Go 和 Node</u>。
- NewAPI 补丁在本地 WSL 里构建镜像，再上传到服务器。
- 补丁放在 clansty 仓库的 `nam-patches` 分支，每个补丁一个提交并带测试。

### 许可证

- clansty 的 new-api 是 **AGPL-3.0**。
- nam 嵌入编译后与它成为**同一个程序**。
- <u>自己使用没有问题</u>；一旦发布，或提供给别人使用，就要履行 AGPL 义务（例如提供对应源码）。
