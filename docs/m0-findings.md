# M0 实测结论

## 总览

**环境**：clansty `d292f5e`（原始提交，worktree `temp/clansty-m0`），WSL docker 中运行，SQLite，`ERROR_LOG_ENABLED=true`；mock 上游同在 compose 网络内（`http://mock:9100`）。脚本在 `docs/m0/*.ps1`（统一用 PowerShell，公共函数在 `_lib.ps1`，令牌从 `temp/m0-env/token.txt` 读取，不进文档）。

- **①type=58 透传**：路径、认证头都对；但 **Gemini 原生格式经 type=58 不按上游 usage 计费**（日志 prompt=2、completion=0）。  
  影响：Gemini 厂商不能直接用 type=58，需要补丁 P5 或改用 type=24。
- **②创建渠道不返回 ID**：已确认；请求必须带 `{"mode":"single","channel":{...}}`，搜索按名称恰好 1 条。  
  影响：与计划一致，搜索结果要再做名称精确匹配（接口是模糊匹配）。
- **③PUT 零值**：`priority=0`、`weight=0` **能写入**；`models=""`、`status=0` **被忽略**。  
  影响：计划 §12 的说法要改：只有字符串和普通 int 字段不能清零，指针字段可以。
- **④默认价格**：`model_ratio` 基本等于原厂价；缺口在**缓存倍率和个别补全倍率**，以及**未带日期的 Claude 别名没有价格（400）**。  
  影响：**P2 不需要**；偏差检查要按字段（输入/补全/缓存）分别比，不要比整次请求的费用。
- **⑤只测被禁用渠道**：实测成立，401 自动禁用后约 121 秒恢复，只测了被禁用的渠道。  
  影响：计划 §3 的配置可用，还要同时开 `AutomaticDisableChannelEnabled`。
- **⑥frt**：非流式固定 `-1000`；流式是毫秒；`use_time` 只有整秒。  
  影响：P1 必需，与计划一致。Claude 流式的 frt 只反映首个事件，不是首字。
- **⑦sub2api**：跳过：待主人提供账号。
- **⑧价格库**：LiteLLM 与 models.dev 在 14 个共有模型上数值完全一致；**选 LiteLLM 为主**，models.dev 作交叉校验。  
  影响：两者都缺较老的 Claude（opus-4-1、sonnet-4），这些要靠手动覆盖。
- **⑨`/nam/` 路由**：**已验证**（`nam-patches` 分支本地二进制）：`/nam/`、`/nam/channels/3`（SPA 深链）返回 nam 的 index.html，`/api/nam/ping` 正常，`/` 仍是 NewAPI 首页。  
  影响：nam 路由必须在 `router.SetRouter` **之前**注册（已在 `main.go` 中这样做）。
- **⑩journal_mode**：`delete`（不是 WAL），全仓库没有任何设置它的代码。  
  影响：嵌入模式读日志要短查询；可评估用 `SQLITE_PATH` 开 WAL。
- **⑪写入副作用**：已逐项列出；**新增渠道（单个/批量）没有调用 `InitChannelCache`**。  
  影响：嵌入适配器新增渠道后必须自己调用；改选项后必须调用 `InvalidatePricingCache`。
- **⑫厂商分组 + auto**：落点、日志 `group`、扣费都正确；**`UserUsableGroups` 里必须同时有 `auto`**，否则 403。  
  影响：计划 §8 的写入清单要加一条 `auto`。
- **⑬日志按渠道过滤**：`channel` 参数可用；`page_size` 上限 100（静默截断）；没有 `min_id`。  
  影响：P4 仍有意义；每次请求都会做 COUNT。

**mock 上游的 bug（已修）**：`/_calls` 里的 `status` 原来永远是 200（`defer` 注册时就求值了状态码），已改成闭包，并加了测试 `TestCallsRecordInjectedStatus`。

## 各项详情

### type=58 透传（①）

**步骤**：用 `sk_default` 令牌对同一个 type=58 渠道发 chat、messages、generateContent，读 mock 的 `/_calls`。脚本 `01-passthrough.ps1`。

**请求与结果**（上游 key 是 `upstream-key-A`）：

```
POST /v1/chat/completions                      -> mock 收到 Authorization: Bearer upstream-key-A
POST /v1/messages                              -> mock 收到 x-api-key: upstream-key-A
POST /v1beta/models/test-model:generateContent -> mock 收到 x-goog-api-key: upstream-key-A
```

- **路径**原样保留，三条都与计划一致。
- **认证头**由 NewAPI 按请求格式重写：Claude 用 `x-api-key`，Gemini 用 `x-goog-api-key`（`relay/channel/passthrough/adaptor.go:134-146`），OpenAI 用 Bearer。
- 客户端无论用 `x-api-key` 还是 `Authorization: Bearer` 调 `/v1/messages`，上游收到的都是 `x-api-key`。
- 客户端用 `?key=sk-...` 调 Gemini，上游路径里**没有**这个 query，key 被换成了 `x-goog-api-key`。

**计费问题（重要）**：

```
messages  ->  日志 prompt=800 completion=500 quota=5250   （正确：800×1.5+200×1.5+500×7.5）
chat      ->  日志 prompt=1000 completion=500 quota=5250  （正确）
generateContent / streamGenerateContent -> 日志 prompt=2 completion=0 quota=3  （错误）
```

- mock 的 Gemini 响应带 `usageMetadata`（1000/200/500），但日志里是按请求文本估算的 2/0。
- 原因：透传适配器对 Gemini 格式用 OpenAI 的处理器解析响应（`adaptor.go:236-243`），不认识 `usageMetadata`。仓库里已有 `gemini.GeminiTextGenerationHandler`（`relay/channel/gemini/relay-gemini-native.go:20`）。

**结论**：type=58 对 **OpenAI 和 Claude 格式可用**；**Gemini 原生格式透传可用但计费错（少收）**。

**对 nam 的影响**：

- Gemini 厂商的渠道<u>不能直接用 type=58</u>。两个选择：新增补丁 P5（让透传适配器对 Gemini 调用原生处理器，约几行），或 Gemini 渠道改用 type=24。
- 这条会影响计划 §3 "渠道一律用 type=58" 的说法，需要主人决定。
- 让 Gemini 客户端走 OpenAI 兼容端点则不受影响（chat 路径计费正确）。

### 创建渠道拿 ID（②）

**步骤**：`POST /api/channel/` → `GET /api/channel/search?keyword=<唯一名>&id_sort=true`。脚本 `02-create-find-id.ps1`。

```
POST /api/channel/   {"name":"nam-m0-xxxx","type":58,"key":"...","base_url":"http://mock:9100", ...}
-> {"message":"channel cannot be empty","success":false}        # 缺 mode/channel 外壳

POST /api/channel/   {"mode":"single","channel":{...}}
-> {"message":"","success":true}                                # 没有 id

GET /api/channel/search?keyword=nam-m0-xxxx&id_sort=true
-> {"data":{"items":[{"id":2,"name":"nam-m0-xxxx", ...}],"total":1,"type_counts":{"58":1},"collapsed_items":[]},"success":true}
```

- **必须的请求形态**：`{"mode":"single","channel":{...}}`（`controller/channel.go:623-629, 671-674`）。`mode` 还可以是 `batch`（key 按换行拆成多个渠道）、`multi_to_single`、`group`。
- **必填**：`key` 不能空（`channel.go:540`）；type=58 必须有 `base_url`（`channel.go:514`）。
- 单个/批量创建的响应里**没有 id**（`channel.go:800-803`）；只有 `group` 模式返回 id（`channel.go:774`）。
- 搜索结果中 `key` 字段恒为空串（不回传密钥）。
- 搜索是**模糊匹配**，所以拿到结果后必须再按 `name` 精确过滤；唯一名（带短 ID）下恰好 1 条。

**结论**：**计划里的做法可行**，实测搜索到恰好 1 条精确匹配。

**对 nam 的影响**：

- 渠道名按计划 `nam-<上游>-<令牌>-<厂商>-<短ID>`，创建后搜索并精确匹配 `name`，再取 `id`。
- 批量创建时用 `mode=single` 逐个创建最稳（`batch` 模式用 key 拼名字时会追加 key 前缀，不可控）。
- 嵌入模式直接用 `model.BatchInsertChannels`，创建后 `channel.Id` 会被 GORM 回填，不需要搜索。

### PUT 写入零值（③）

**步骤**：对已有渠道依次 `PUT /api/channel/` 发 `{"id":X,"priority":0}`、`{"id":X,"models":""}` 等，每次读回。脚本 `03-put-zero.ps1`。

```
初始            priority=3 weight=7 models=[7个模型] status=1
PUT priority=0  -> priority=0 （写入成功）  models 保持不变
PUT models=""   -> models 保持不变            （被忽略）
PUT weight=0    -> weight=0   （写入成功）
PUT status=0    -> status=1   （被忽略）
PUT status=2/1  -> 正常变化
```

- 更新用 `DB.Model(channel).Updates(channel)`（`model/channel.go:556`），GORM 的 struct 更新**忽略零值字段**。
- `Priority` 是 `*int64`、`Weight` 是 `*uint`（`model/channel.go:467-479`），指针非 nil 时 0 会被写入。
- `Models`（string）、`Status`（int）是普通类型，零值被忽略。
- 更新后会重建该渠道的全部 abilities（`channel.go:561` → `ability.go:202`）。

**结论**：**priority=0 变成 0；models 保持不变**；weight=0 也能写入；status=0 无效。

**对 nam 的影响**：

- 计划 §12 的"PUT 不能把非指针字段清零（status=0、models 为空）"是对的，但要补一句：<u>priority、weight 可以清零</u>。
- 改状态只用 1/2/3；要清空模型，只能改成一个占位模型名或删除重建。
- 嵌入模式如果直接调用 `channel.Update()`，行为完全相同；想真正写零值需要用 `Select()` 自己写 SQL，不建议。

### 默认价格与原厂价（④）

**步骤**：不写任何价格配置（只给 `test-model` 配了价格用于 ①），对 12 个常用模型各发一次，读日志 `other` 与官方价（LiteLLM）对比。脚本 `04-default-prices.ps1`，样本 `temp/m0-env/litellm.json`。

mock 的固定用量折算：非缓存输入 800 + 缓存读 200 + 输出 500。

```
模型                         model_ratio  completion  cache_ratio  NewAPI(USD)  官方(USD)   差异
claude-sonnet-4-5-20250929   1.5          5           0.1          0.00996      0.00996      0
claude-opus-4-5-20251101     2.5          5           0.1          0.0166       0.0166       0
claude-haiku-4-5-20251001    0.5          5           0.1          0.00332      0.00332      0
claude-opus-4-1-20250805     7.5          5           0.1          0.0498       0.0498*      0
gpt-4o                       1.25         4           0.5          0.00725      0.00725      0
gpt-5                        0.625        8           0.1          0.006026     0.006025     0
gpt-4.1-mini                 0.2          2           0.25         0.00074      0.00114     -35%
gemini-2.5-pro               0.625        8           1(无折扣)    0.00625      0.006025    +3.7%
gemini-2.5-flash             0.15         8.33        1(无折扣)    0.00155      0.001496    +3.6%
o3                           1            4           1(无折扣)    0.006        0.0057      +5.3%
deepseek-chat                0.135        1           0.25         0.000364     0.00044     -17%
claude-sonnet-4-5 / claude-opus-4-1（无日期别名）  -> HTTP 400 "价格未配置"
```

\* LiteLLM 里没有 opus-4-1；按官方价 15/75/1.5 手算也是 0.0498。

- **输入倍率**：除 deepseek 外全部等于原厂价。
- **补全倍率**：`claude-*`、`gpt-5`、`o3`、`gemini-2.5-*` 是代码里硬编码并**锁定**的（`setting/ratio_setting/model_ratio.go:501-595`），且与官方一致。
- **gpt-4.1-mini 的补全倍率偏低**：硬编码为 2（`model_ratio.go:539` 对 gpt-4 系的默认值，且**未锁定**），官方应为 4，所以输出少收一半。
- **缓存倍率**：Claude 是 0.1 / 建缓存 1.25，与官方 5 分钟档一致。`gemini-2.5-*`、`o3` 没有缓存倍率（等于 1，不打折），所以缓存部分多收。1 小时档（2 倍）没有验证。
- **deepseek-chat**：默认价是旧价（0.27 / 输出同倍率 1），LiteLLM 现在是 0.28 / 0.42。
- **未带日期的 Claude 别名**（`claude-sonnet-4-5`）没有价格，请求被拒（400，错误码 `model_price_error`）；只有带日期的 key 有价格。
- 自用模式开启时，未配置价格的模型会按 37.5 倍率放行（`model_ratio.go:416`）。

**结论**：NewAPI 默认的模型价格**大体等于原厂价**；偏差集中在缓存倍率、个别补全倍率、过时模型和无日期别名。**补丁 P2 不需要**。

**对 nam 的影响**：

- **P2 不做**：会出错的 `gpt-4.1-mini` 补全倍率是**未锁定**的，直接写 `CompletionRatio` 配置表就能覆盖；被锁定的那些本来就对。
- 偏差检查按字段分别比较（输入倍率、补全倍率、缓存读、缓存写），不要比整次请求的费用，否则缓存的偏差会被摊淡；5% 阈值能抓到上表所有偏差。
- 检查"缺价模型"：某渠道的模型名在 NewAPI 没有价格时，请求会 400，nam 应当告警。
- 模型名需要别名映射（`claude-sonnet-4-5` → `claude-sonnet-4-5-20250929`）。

### 自动恢复（⑤）

**步骤**：开启 `AutomaticDisableChannelEnabled` 和 `AutomaticEnableChannelEnabled`，`auto_test_channel_enabled=false`，`auto_test_channel_minutes=1`；两个渠道（key A / B）；mock 返回一次 401；恢复后轮询。脚本 `05-auto-recover.ps1`。

```
初始            ch1=1 ch2=1
请求(mock 401)  客户端收到 401；ch1=3(自动禁用) ch2=1
恢复 mock 03:36:45  ->  03:38:41 NewAPI 对 ch1 发出测试请求(key A)，ch1 回到 1
恢复耗时 121 秒；这段时间 /_calls 里只有 1 条，key 是 A，ch2(key B)没有被测
```

- 401 能自动禁用，是因为默认 `AutomaticDisableStatusCodes=401` 且开了 `AutomaticDisableChannelEnabled`（`service/channel.go:77-107`）。渠道变成状态 3，并写一条 type=5 的错误日志。
- 调度循环（`controller/channel-test.go:1104-1139`）：全量自动测试关闭、自动恢复开启时，只调 `testAutoDisabledChannels`（1133-1135）。
- 被测对象的判定在 `shouldTestChannelAutomatically`（989-1000）：手动禁用(2)不测；自动禁用(3)且渠道的 `auto_recover=1`（默认）才测；启用中的渠道在"只测被禁用"范围里直接跳过。
- 恢复条件 `ShouldEnableChannel`（`service/channel.go:116-125`）：测试成功、状态是 3、`AutomaticEnableChannelEnabled` 为真。
- 间隔来自 `auto_test_channel_minutes`，**最小 1 分钟**（`setting/operation_setting/monitor_setting.go:67-76`）。
- 121 秒的构成：两个开关原先都关着时循环每 1 分钟才重新读一次配置（`channel-test.go:1117-1122`），读到后再等一个间隔才测试，所以**最坏约 2 个间隔**。
- 客户端那次请求直接收到 401，没有重试到 ch2：默认 `RetryTimes=0`。

**结论**：**成立**。只测被禁用渠道，禁用→恢复在一个测试周期（加上最多一次配置重读）内完成。

**对 nam 的影响**：

- 计划 §3 写的配置可用，补充：必须同时开 `AutomaticDisableChannelEnabled`，否则不会自动禁用。
- 恢复测试用的是渠道的 `test_model`，该模型必须有价格，且恢复测试会产生一条"模型测试"日志（统计时要排除，见 ⑬）。
- 想缩短恢复时间，把间隔设为 1 分钟即可，不能更低。
- `RetryTimes` 默认 0：一次 401 会直接返回给用户，故障转移要靠 M8 另行设计。

### 非流式与流式的 frt（⑥）

**步骤**：分别发非流式、流式请求，读日志 `other.frt` 与 `use_time`。脚本 `06-frt.ps1`。

```
路径                                   is_stream  use_time  frt
/v1/chat/completions                   false      0s        -1000
/v1/messages                           false      0s        -1000
/v1/chat/completions                   true       2s        303
/v1/messages                           true       2s        22
/v1beta/.../streamGenerateContent      true       3s        304
```

- 非流式 `frt` 恒为 `-1000`（常量），不是耗时。
- 流式 `frt` 是毫秒；mock 的首字延迟是 300ms，chat 和 Gemini 的 303/304 正好对上。
- Claude 流式的 22ms：mock 先发 `message_start` 事件，再等 300ms 发首个内容，所以 `frt` 记的是**首个 SSE 事件**，不是首个内容 token。真实的 Anthropic 也会先发 `message_start`，所以 Claude 流式 `frt` 会偏小。
- `use_time` 只有整秒（2.4 秒的流被记成 2s 或 3s）。

**结论**：**非流式 frt = -1000**，流式是毫秒。

**对 nam 的影响**：

- P1 必需，与计划一致：`use_time_ms` 加毫秒耗时，非流式不再写 `-1000`。
- 统计首字延迟时必须排除 `frt<=0`。
- Claude 流式的首字延迟要么不统计，要么在 P1 里改为"首个内容块"；计划 M4 的"p50 在 250–450ms"断言要用 chat 路径的请求，不能用 messages 流。

### sub2api 余额与分组接口（⑦）

跳过：待主人提供账号。

- 脚本 `07-sub2api.ps1` 只是占位。
- 待测内容：登录、余额、分组接口的路径与响应字段；如果不可用，M3 第 5 步按计划标为"手动"。

### 价格库字段映射（⑧）

**步骤**：下载 LiteLLM（4772 条）和 models.dev（235 个厂商）的数据，对 20 个常用模型对照。脚本 `08-price-sources.ps1`，样本在 `temp/m0-env/`。

```
字段                   LiteLLM                                  models.dev
输入价                 input_cost_per_token (USD/token)         cost.input (USD/百万token)
输出价                 output_cost_per_token                    cost.output
缓存读                 cache_read_input_token_cost              cost.cache_read
缓存写 5 分钟          cache_creation_input_token_cost          cost.cache_write
缓存写 1 小时          cache_creation_input_token_cost_above_1hr  （没有）
按请求/搜索            search_context_cost_per_query 等          （没有）
长上下文分档           （另有 *_above_200k_tokens 字段）          cost.tiers / context_over_200k
```

- **共有模型数值完全一致**：claude-sonnet-4-5 / opus-4-5 / haiku-4-5、gpt-4o、gpt-4o-mini、gpt-4.1 系列、gpt-5、gpt-5-mini、o3、o4-mini、gemini-2.5 三款，共 14 个，输入/输出/缓存读/缓存写四项全部相同。
- **LiteLLM 缺**：claude-opus-4-1、claude-sonnet-4（无日期和带日期的 key 都没有）、claude-3-5-haiku。
- **models.dev 缺**：同样没有 opus-4-1、sonnet-4、3-5-haiku；`deepseek-chat` / `deepseek-reasoner` 也没有（现在是 `deepseek-v4-*` 等新名字）。
- LiteLLM 的 key 有厂商前缀（4090 个含 `/`），Gemini、DeepSeek 要用 `gemini/gemini-2.5-pro`、`deepseek/deepseek-chat`，需要别名映射表。（第 26 轮修订：只接受原厂前缀，转售商前缀的条目不用。）
- models.dev 按厂商分层、单位是 USD / 百万 token，结构更干净，但字段少。
- LiteLLM 的缓存写只有 5 分钟档和"1 小时以上"档，OpenAI / Gemini 没有缓存写价（NewAPI 也不需要）。
- 按请求计费（图片、搜索）的字段没有逐个核对，留到 M3。

**结论**：**主数据源选 LiteLLM**，models.dev 用来交叉校验。

**对 nam 的影响**：

- 理由：LiteLLM 有 1 小时缓存写价和按请求/搜索的价格，覆盖面大（4772 条），且 DeepSeek 等仍有数据。
- 导入时统一换算成 USD / 百万 token；两个源都缺的模型靠"手动覆盖"，状态显示"缺价"。
- 两源不一致时（本次抽样为 0）取 LiteLLM 并记录差异，不自动写入 NewAPI。
- 价格库每次同步先写 `official_price`，再与 NewAPI 当前倍率比对（见 ④ 的按字段比较）。

### `/nam/` 路由不被拦截（⑨）

待嵌入镜像完成后验证。

- 脚本 `09-nam-route.ps1` 只是占位。
- 待测内容：`GET /nam/ping`、`GET /api/nam/ping`、`/nam/xxx` 深链，确认 `static.Serve("/")` 和 `NoRoute` 不拦截。

### SQLite 的 journal_mode（⑩）

**步骤**：容器内 SQLite 文件由 NewAPI 创建；从宿主机只读打开读 `PRAGMA journal_mode`，并检查文件头。脚本 `10-sqlite-journal.ps1`。

```
data/one-api.db   文件头第 18/19 字节 = 01 01（非 WAL）
PRAGMA journal_mode -> ('delete',)
```

- **是 `delete` 模式，不是 WAL**。
- 代码里 DSN 是 `one-api.db?_busy_timeout=30000`（`common/database.go:15`），可以用环境变量 `SQLITE_PATH` 覆盖（`common/init.go:64-65`）。
- 仓库里**没有任何地方设置 `journal_mode` 或 `PRAGMA`**（只有两处 `PRAGMA table_info` 查询）。
- 连接池上限 `SQL_MAX_OPEN_CONNS` 默认 1000（`model/main.go:283`）。
- `delete` 模式下，写者要等所有读者结束；写事务期间其他连接读会阻塞，最长等 30 秒（`busy_timeout`）。

**结论**：**journal_mode = delete，没有 WAL**。

**对 nam 的影响**：

- 嵌入模式读 NewAPI 日志要用**短查询、分页**（每次 ≤100 行），不要长事务；nam 自己的表放 `nam.db`，不要和 NewAPI 抢同一个文件。
- 写渠道和选项是低频小事务，30 秒的等待时间足够，风险较低。
- 可以评估把 `SQLITE_PATH` 设为带 WAL 的 DSN（modernc 驱动一般支持 `_pragma=journal_mode(WAL)`），但**没有实测**，M1.5 嵌入后再测；不建议在没测之前改生产配置。

### 写入副作用清单（⑪）

**步骤**：读源码（`temp/clansty-m0`，`d292f5e`），列出除写库以外的全部副作用。脚本 `11-write-side-effects.ps1` 会打印这些行号。

**新增渠道**（`controller.AddChannel`，`controller/channel.go:663-805`）：

- 校验 `validateChannel(isAdd=true)`（`channel.go:505-589`）：key 非空、type=58 要有上游地址、渠道设置 JSON 合法、模型名 ≤255 字符、倍率和超时不能为负。
- 解析 sub2api 登录状态：`resolveSub2APIAuthUpdate` / `ApplySub2APIState`（692-697）。
- 写库：`model.BatchInsertChannels`（`model/channel.go:412-439`）在**同一个事务里**插入渠道并调用 `AddAbilities`（`model/ability.go:155-194`，按"分组×模型"展开）。
- 清代理连接缓存：`service.ResetProxyClientCache()`（`channel.go:799`，`service/http_client.go:74`）。
- **没有** `model.InitChannelCache()`（只有 `group` 模式有，772 行）。

**更新渠道**（`controller.UpdateChannel`，`channel.go:1068-1232`）：

- 校验 `validateChannel(isAdd=false)`，读回原渠道，保留 `ChannelInfo`（多 key 状态）、沿用原 `Setting`。
- 多 key 的"追加/覆盖"密钥处理（1124-1202）。
- 写库：`channel.Update()`（`model/channel.go:516-563`）= `Updates`（忽略零值）+ **重建 abilities**（`UpdateAbilities`，先删后建）。
- 保存 sub2api 状态、手动上游倍率（1208-1217）。
- `model.InitChannelCache()`（1218）、`service.ResetProxyClientCache()`（1219）、`service.CancelChannelAffinityForce(id)`（1220，取消渠道强制吸附）。
- 响应里 `key` 被清空。

**删除渠道**（`controller.DeleteChannel`，`channel.go:807-822`）：

- `CancelChannelAffinityForce(id)` → `channel.Delete()`（`model/channel.go:614-624`，事务内删成员、abilities、渠道） → `model.InitChannelCache()`。
- **没有** `ResetProxyClientCache()`。

**写选项**（`controller.UpdateOption` → `model.UpdateOption`）：

- 控制器层校验（`controller/option.go:135-318`）：`GroupRatio` 走 `CheckGroupRatio`（只检查 JSON 合法与不为负）；`CreateCacheRatio`、`ImageRatio` 等会**提前**调用更新函数；开关类选项有前置条件检查。`AutoGroups`、`UserUsableGroups`、`ModelRatio`、`CompletionRatio`、`CacheRatio` **没有**控制器层校验。
- `model.UpdateOption`（`model/option.go:220-234`）：先 `FirstOrCreate` + `Save` 写库（**两次 DB 调用的错误都被忽略**），再 `updateOptionMap`。
- `updateOptionMap`（236-571）：写 `common.OptionMap`（加锁）→ 按 key 更新内存变量：`GroupRatio`（517）、`UserUsableGroups`（521）、`AutoGroups`（369）、`Price`/`USDExchangeRate`（379-382）、`ModelRatio`（515）、`CompletionRatio`（523）、`CacheRatio`（531）、`CreateCacheRatio`（533）等。
- 非法 JSON：`LoadFromJsonString` 先清空 map 再反序列化（`types/rw_map.go:77-82`），所以<u>非法值会清空运行时价格表</u>，而且这时数据库里已经存了非法值。
- **不会**刷新定价页缓存：`InvalidatePricingCache()` 只在 `billing_setting.*` 配置变化时被调用（`model/option.go:601`）。定价页缓存 TTL 1 分钟（`model/pricing.go:67`）。
- `ModelRatio`/`CompletionRatio`/`CacheRatio` 等的更新函数带回调 `InvalidateExposedDataCache`（`model_ratio.go:394`）；`GroupRatio` 没有。
- 多实例/重启：`SyncOptions` 每 `SYNC_FREQUENCY`（默认 60 秒）从数据库重载（`model/option.go:205-218`）。

**内存缓存开关**：`MEMORY_CACHE_ENABLED` 默认关闭（`common/init.go:83`，Redis 开启时自动开，`main.go:64-66`）。开启后选渠道读内存缓存，`SyncChannelCache` 每 60 秒重载（`model/channel_cache.go:96`）。本次测试环境没有开启，所以"新增后立即可用"在开启内存缓存的情况下**没有实测**，是源码推断。

**结论**：嵌入适配器必须复用或复现下面这些步骤。

**对 nam 的影响**：

- **新增渠道**：校验 → `BatchInsertChannels`（含 abilities）→ `ResetProxyClientCache()` → **自己加上 `model.InitChannelCache()`**（原控制器漏了）。
- **更新渠道**：校验 → `channel.Update()`（含重建 abilities）→ `InitChannelCache()` → `ResetProxyClientCache()` → `CancelChannelAffinityForce()`。
- **删除渠道**：`CancelChannelAffinityForce()` → `channel.Delete()` → `InitChannelCache()`。
- **写选项**：先在 nam 里完整校验 JSON（不要依赖控制器）→ `model.UpdateOption` → `InvalidatePricingCache()` → 读回核对。`model.UpdateOption` 本身不返回写库错误，所以读回是唯一可靠的确认。
- 写 `AutoGroups`/`UserUsableGroups`/`GroupRatio` 之后建议同时调用 `ratio_setting.InvalidateExposedDataCache()`。

### 厂商分组 + auto 令牌（⑫）

**步骤**：写 `GroupRatio`（加 `anthropic:0.05`）、`AutoGroups=["anthropic","default"]`、`UserUsableGroups`；建渠道 2（`group=anthropic`，key `ANTH`）；建 `group=auto` 令牌；对 `test-model` 与 `gpt-4o` 各发一次。脚本 `12-vendor-group-auto.ps1`。`test-model` 价格仅本次测试用（`ModelRatio=1.5`，`CompletionRatio=5`）。

```
第一次尝试：令牌 group=auto，UserUsableGroups 只含 default/vip/anthropic
-> HTTP 403 {"message":"无权访问 auto 分组"}

把 auto 加进 UserUsableGroups 后：
test-model via auto -> mock 收到 Bearer upstream-key-ANTH，日志 channel=2 group=anthropic
                       quota=263  group_ratio=0.05  model_ratio=1.5  completion_ratio=5
gpt-4o via auto     -> mock 收到 Bearer upstream-key-A，日志 channel=1 group=default
                       quota=3625 group_ratio=1
```

**计算**（`test-model`，缓存倍率为 1，等于无折扣）：

```
基础额度 = 1000 × 1.5 + 500 × 1.5 × 5 = 1500 + 3750 = 5250
quota    = 5250 × GroupRatio[anthropic] 0.05 = 262.5  → 日志 263（取整）
```

- 对比同模型在 `default` 组（倍率 1）的日志：5250。所以 anthropic 的实际价格是 default 的 5%，与 `GroupRatio` 完全吻合。
- **落点**：`AutoGroups` 里第一个对该模型有可用渠道的组（`service/channel_select.go:94-131`）。`test-model` 只有 anthropic 组有 → 落 anthropic；`gpt-4o` 只在 default 组 → **自动回落到 default**。
- **日志 `group` 字段**是实际选中的分组（`anthropic` / `default`），不是 `auto`。
- **扣费**用选中的分组：`relay/helper/price.go:23-60` 的 `HandleGroupRatio`；`GroupGroupRatio`（用户分组×使用分组）如果配了会优先。
- 令牌分组检查（`middleware/auth.go:386-402`）：令牌的 group 必须在该用户可用分组里，**`auto` 自己也必须在 `UserUsableGroups` 里**；`auto` 不需要出现在 `GroupRatio` 里（399-403 对 auto 豁免）。
- 默认的 `UserUsableGroups` 是 `{"default":..., "vip":...}`，不含 `auto`。

**结论**：**成立**。请求落到 anthropic 分组的渠道，日志 `group` 是 `anthropic`，扣费按 `GroupRatio[anthropic]` 计算（262.5 → 263）。

**对 nam 的影响**：

- **计划 §8 的写入清单要加一条**：`UserUsableGroups` 必须同时包含 `auto` 和各个厂商分组；否则 auto 令牌会 403。
- 回落行为是好事：厂商组里没有该模型时，会落到 `AutoGroups` 后面的组，但价格就按后面组的倍率算 —— 所以"一个模型只属于一个厂商分组"的校验仍然必要，且 `default` 不应再放这些模型。
- 一致性检查可以加一条：`AutoGroups` 里的厂商分组都必须在 `GroupRatio` 里；`auto` 必须在 `UserUsableGroups` 里。
- 渠道 `group` 字段是逗号分隔（`ability.go:156-157` 按 `,` 拆），所以"恰好等于所属厂商分组"要检查整个字符串，不能用包含判断。

### 日志按渠道过滤（⑬）

**步骤**：`GET /api/log/?channel=<id>&start_timestamp=...`，另测分页与参数上限。脚本 `13-log-by-channel.ps1`。

```
GET /api/log/?p=1&page_size=10&type=2&channel=2&start_timestamp=<1小时前>
-> {"data":{"page":1,"page_size":10,"total":1,"items":[{ ... "channel":2, ... }]},"success":true}

字段: id,user_id,created_at,type,content,username,token_name,model_name,quota,prompt_tokens,
      completion_tokens,use_time,is_stream,channel,channel_name,token_id,token_root_id,group,
      ip,request_id,user_agent,other
```

- **按渠道过滤**：参数名 `channel`（整数，`controller/log.go:13-26`），SQL 是 `logs.channel_id = ?`（`model/log.go:156-190`）。其他参数：`p`、`page_size`、`type`、`start_timestamp`、`end_timestamp`、`username`、`token_name`、`model_name`、`group`、`request_id`。
- **分页**：`p` 从 1 开始；排序固定 `id desc`；每次请求都先做一次 `COUNT(*)`（`model/log.go:187`）；越界页返回空数组但 `total` 仍正确。
- **`page_size` 上限 100**，超过的值（101、1000、100000）都被**静默截断**为 100，不报错（`common/page_info.go` 的 `PageSize > 100`）。默认 10。
- 不带 `type` 会混入 type=5（错误日志），本次 `channel=1` 返回 26 条消费日志 + 1 条错误日志。
- **没有 `min_id`**：传 `min_id=9999` 被忽略，照样返回数据。所以 P4 仍然需要（增量拉取靠 `id desc` + 时间戳去重不够稳）。
- 测试日志（渠道自动测试 / 手动测试）也是 type=2，`content` 是"模型测试"（语言相关），**没有专门的标记字段**。M4 需要找可靠的判别方式（例如 `token_id=0` 或 `token_name` 为空），本次没有核实。
- `other` 是 JSON 字符串，含 `model_ratio`、`completion_ratio`、`cache_ratio`、`group_ratio`、`frt`、`cache_tokens`、`request_path` 等；Claude 请求还有 `cache_creation_ratio`。

**结论**：**独立模式可以按渠道过滤**，返回字段足够做统计；分页最多 100 条/页。

**对 nam 的影响**：

- 独立模式拉日志：按 `channel` + 时间范围分页，`page_size=100`，每次多一次 COUNT；渠道多时请求数多，注意限速。
- 增量拉取：没有 `min_id`，要么做 P4，要么用 `start_timestamp` 加 `request_id` 去重。嵌入模式直接按 `id` 查库，不受影响。
- 统计时必须排除测试日志，并且要单独处理 type=5 的错误日志（失败次数的来源）。

