# nam 项目文档索引与开工指引

nam（newapi-manager）是给 clansty 版 NewAPI 用的上游管理工具。**现在处于"需求和计划已定，尚未开始编码"的阶段**，下一步是 M0 实测。

## 文档

### 先读哪些

- **`docs/requirements.md`：需求，冲突时以它为准**  
  §3 是每一轮对话的原文和结论，§4 是当前有效需求（带编号和状态），§6 是待确认问题。
- **`.omo/plans/newapi-manager-plan.md`：实施计划**  
  架构、数据表、适配器、统计、定价、里程碑和每个里程碑的 QA 场景。已经过 Momus 审查（OKAY）。
- **`docs/feature-split.md`：功能划分**  
  哪些改 NewAPI 代码、哪些只改配置、哪些放在 nam 核心、两个适配器各做什么。
- **`docs/ui-features.md`：界面功能说明**  
  每个页面的表格列、按钮、弹窗和用户逻辑。
- **`docs/ui/mockup.html`：界面原型**  
  单文件 HTML，浏览器直接打开。视觉风格以它为准。`docs/ui/sitemap.svg` 是页面结构图。
- **`docs/research/new-api-fork-report.md`：clansty 分支调研**  
  clansty 与官方 new-api 的差异、同步状态和自有功能。

### 不在 git 里的参考资料

- **`temp/clansty-new-api`**：clansty 分支源码（HEAD d292f5e8），补丁和嵌入适配器都基于它。
- **`temp/metapi`**：调研过的替代方案源码，只作参考。
- **`temp/ui-src`**：原型的早期分片草稿，<u>比 `mockup.html` 旧</u>。它的 `build.js` 已改为只输出到草稿目录，不要再用它覆盖原型。
- **`F:\Mess\oc-modal-configurator`**：表格框选、复制粘贴逻辑的复用来源（`selection/machine.ts`、`useSpreadsheetSelection.ts`）。

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
- **前端独立**：React + TypeScript，挂在 `/nam/` 下，不改 NewAPI 现有前端，不用 iframe。接口统一用 `/api/nam/` 前缀。

## 新会话怎么开工

### 下一步：M0 实测

- 在本地 docker 里运行 clansty 版 NewAPI，按计划 §11.1 的 M0 表逐项实测 11 项，结论写进 `docs/m0-findings.md`。
- 其中第 ⑦ 项要用一个真实的 sub2api 上游，<u>需要主人提供账号</u>。
- M0 的结论会决定补丁 P2 要不要做（Q3），以及嵌入模式写 NewAPI 时要注意什么（Q5、⑪）。

### 还没定的事

- **需要主人确认的**  
  售价倍率 m 的单位：计划里定成"元 / 官方美元"，和渠道成本系数同一个单位。需求文档 P1 原来的写法里 m 不含汇率，开工前要确认。
- **要另外讨论的**  
  优先级算法和故障转移（需求 F3、计划 §9），本期只留接口。

### 环境约定

- 临时文件放在 `temp/`；用到 Python 时在项目里建 venv。
- NewAPI 补丁在本地 WSL 里构建镜像，再上传到服务器。
- 补丁放在 clansty 仓库的 `nam-patches` 分支，每个补丁一个提交并带测试。
