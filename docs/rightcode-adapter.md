# Right Code 上游适配

Right Code（https://rightapi.io）是一个自研的 AI 中转站，**不是 NewAPI，也不是 sub2api**。本文记录探索结论和编码计划，供后续编码的 Agent 直接照做。

- **原始探索报告**在 `temp/rightcode/`（已被 `.gitignore` 忽略）：
    - `public-endpoints.md`：免登录接口实测
    - `auth-endpoints.md`：登录后 GET 接口实测
    - `user-api.md`、`chunks-api.md`：前端 JS 静态分析
- ⚠️ `temp/rightcode/raw-auth/` 里有**明文 user_token 和 API Key**。拿来做测试数据时<u>必须先打码</u>，不能提交进仓库。

## 探索结论

### 站点基本形态

- **后端是 Spring Boot（Java）**，前端是 Vite + React。
- **接口没有 `/api` 前缀**，直接挂在根上：`https://rightapi.io/auth/me`。
    - `/api/status`、`/api/user/self` 都返回 404，可以用来和 NewAPI 区分。
- **成功响应是裸 JSON**，没有 `{success, data, message}` 外壳，成败只看 HTTP 状态码。
- **错误响应**是 Spring 默认格式：
    - `{"timestamp","path","status","error","requestId","message"}`，文案在 `message`。
    - 例外：`/v1/*` 中转接口的错误是 `{"error":"缺少 API Key"}`，`error` 是字符串。
- **状态码含义**：
    - `401`：token 无效或过期。
    - `403`：有两种情况，一是账号被封禁，二是功能无权限（例如 `/api-key/quota-plans` 返回"该功能仅限企业用户使用"）。<u>适配器用到的接口不会出现第二种情况</u>。
- **不需要 cookie**：Cloudflare 没有拦截普通请求，只带浏览器 UA 就能访问。

### 鉴权

- 请求头：`Authorization: Bearer <user_token>`。
    - `user_token` 是 UUID 形式的账号令牌，长 36 位。
    - 获取方式：在浏览器里登录站点，F12 → Application → Local Storage → `userToken`。
    - `GET /auth/me` 的响应里也有 `user_token` 字段，和请求头里的值相同。
- **密码登录不可用**：`POST /auth/login` 需要腾讯验证码（`public.captcha.provider = "tencent"`），只能在浏览器里完成。
    - 请求体是 `{username, password, turnstile_token, otp_code}`，腾讯验证码的 `{ticket, randstr}` 会被序列化成字符串放进 `turnstile_token`。
    - 结论：**适配器只支持 `access_token` 凭证**，内容填 `user_token`。
- **token 轮换**：账号设置里有 `token_rotation_enabled`、`token_rotation_days`（实测账号是开启状态，周期 30 天）。到期后旧 token 很可能失效，会收到 401。
- **禁止调用 `/auth/reset-user-token`**：它会让已经保存的 token 作废。

### 用户与余额

- **`GET /auth/me` → 200**，返回用户对象本身。适配器关心的字段：
    - `id`（数字）、`username`、`email`
    - `balance`：余额，浮点数，6 位小数，例如 `497.478201`
    - `is_banned`、`is_admin`（数值，0 为普通用户）
    - `token_rotation_enabled`、`token_rotation_days`
- **余额单位**：前端显示为 `$xx.xx`，没有除以 100，模型价格也标注 `official_currency: "USD"`。
    - <u>按 USD 处理</u>：`Balance.USD = balance`。
    - 充值是人民币，但站点余额本身按美元记账（`public.balance.price = "1.0"`，含义未完全确认）。
- **不要用的接口**：
    - `/account/summary`：用 user_token 调用返回 401 `Invalid API Key`，它走另一套鉴权。
    - `/subscriptions/*`：订阅额度和余额分开计算，实测账号没有订阅。<u>第一版不处理订阅</u>。

### 渠道（分组）

- **Right Code 的"渠道"就是项目里的"分组"**。每个渠道有一个数字 `id` 和一个路径前缀 `prefix`。
- **`GET /upstreams/public`**（免登录）→ `{ "upstreams": [...] }`，共 13 个。
    - 条目字段：`id, name, prefix, type, supported_protocols[], default_protocol, is_active, chat_enabled, rate, remark`
    - `prefix` 例如 `/codex`、`/claude`、`/gemini`、`/cursor`、`/phm`。
    - `rate` 是渠道倍率，例如 `/codex` 0.4、`/claude` 2.0。JSON 里是数字（`0.20000000`）。
    - `remark` 是 HTML 格式的说明。
- `GET /upstreams/effective`（需登录）的结构与 public 完全相同。实测账号的 rate 也相同，可以优先用它，失败时回退 public。
- `/tiers/public` 是订阅档位的权限表（`deny_prefixes`），**和价格无关，不要用**。

### 模型与价格

- **`GET /models/effective`**（需登录）→ `{ "upstreams": [ 渠道头 + models[] ] }`。
    - 渠道头字段：`upstream_id, name, prefix, effective_upstream_rate, official_currency, copy_with_v1, modality, supported_protocols, default_protocol, visibility, remark, models`
    - 免登录的 `/models/public` 结构基本相同，但渠道头没有 `upstream_id`。
- **模型条目字段**：
    - `model_id, name, is_available, billing_mode`（`"token"` 或 `"tiered"`）
    - 原价（数字）：`input_price, output_price, cache_read_input_price, cache_creation_input_price, request_price`
    - `tiers[]`：按上下文长度分档，每档有 `max_context_length, max_output_length` 和上面四个价格
    - `base_price_config`：原价，值是**字符串**
    - `effective_price_config`：**实际价**（原价 × 倍率），值是字符串
    - `effective_upstream_rate, effective_model_rate, effective_total_multiplier`：字符串
    - `time_periods, param_rules`：时段计价和参数规则，第一版忽略
- **价格单位**：USD / 百万 token。`request_price` 是 USD / 次。
- **实际价的算法**：`实际价 = 原价 × 渠道 rate × 模型 rate`。
    - 例：`/codex` 的 `codex-auto-review`，原价输入 5、输出 30，倍率 0.4，实际价为输入 2、输出 12。
- **`effective_price_config` 有两种形状**，都要支持：
    - 普通：`{"input_price":"2","output_price":"12","cache_read_input_price":"0.2","cache_creation_input_price":"0"}`
    - 分档：`{"tiers":[{"input_price":"2","output_price":"12",...,"max_context_length":272000}]}`，没有扁平字段
- **已知差异**：有 6 个 deepseek 模型带时段计价，在 `/deepseek`、`/flash` 两个渠道下。它们在 effective 里的价格只有 public 的一半，不确定哪个是实际计费价。<u>以 effective 为准</u>，上线后再对账。

### API Key

- **`GET /api-key/list`** → `{ "keys": [...], "total": 2 }`。
    - **不分页**：传 `page`、`page_size` 也会被忽略，一次返回全部。
    - **`key` 是完整明文**（`sk-` 开头），没有单独的"查看完整 key"接口。
    - 条目字段：`id, bound_upstream_id, user_id, key, name, quota_limit, used_quota, expired_at, is_active, allowed_prefixes, allowed_models, allow_wallet, allowed_item_ids, is_system, created_at, updated_at, last_used_at, disabled_by_security_review, plan_id, department_id, ...`
- **字段含义**：
    - `bound_upstream_id`：绑定的渠道 id，对应 `/upstreams/*` 的 `id`。`null` 表示旧式 key，不限渠道。
    - `quota_limit`：**总额度上限**（USD），`null` 表示不限。<u>注意它不是"剩余额度"</u>。
    - `used_quota`：已用额度（USD）。
    - `expired_at`：到期时间，格式 `"2026-11-10T18:28:27"`，没有时区，按**东八区**理解。`null` 表示永不过期。
    - `is_active`、`disabled_by_security_review`（可能为 null）：启用状态。
- **`GET /api-key/{id}`**：返回单个 key，字段与列表一致。
- **`POST /api-key/create`**：请求体如下（来自前端代码，未实测，因为探索时禁止写操作）：

```json
{
  "name": "nam-xxx",
  "bound_upstream_id": 27,
  "allowed_models": [],
  "allow_wallet": true,
  "allowed_item_ids": [],
  "quota_limit": null
}
```

- 关于 create 的几点说明：
    - `bound_upstream_id` 必填，前端校验不能为空。
    - `allow_wallet` 和 `allowed_item_ids` 至少要启用一个，否则前端会拦截。
    - 响应里**只确认有 `id`**。是否返回完整 key 未知，所以创建后要按 id 调 `GET /api-key/{id}` 取 key。
    - 到期时间不在 create 里设置，要单独调 `PATCH /api-key/{id}/expire`，<u>第一版不需要</u>。
- **其他接口（第一版不用）**：
    - 更新是 `PATCH /api-key/{id}`，删除是 `DELETE /api-key/{id}`。
    - `/api-key/options` 返回 `[{id,name}]` 裸数组。
    - `/api-key/chat-keys` 参数没有试出来，一律返回 400。

### 中转与公告

- **中转地址**：渠道由 key 决定，**地址里不加 `/codex` 之类的前缀**。
    - OpenAI / Codex：`https://rightapi.io/v1`
    - Claude Code：`https://rightapi.io`（不加 `/v1`）
    - Gemini：`https://rightapi.io/v1beta/models`
- **`GET /v1/models`**（带 sk- key）→ `{"success":true,"object":"list","data":[{"id":"...","object":"model"}]}`。
    - 只返回该 key 所绑渠道的模型，没有价格字段。
    - 可以直接复用项目里的 `listModelsOpenAI`，它只读 `data[].id`。
- `GET /account/relay-domains` 返回 `{"domains":[{url,name,enterprise}],"default_url":"https://api.site"}`。
    - `api.site` 能不能用来中转还没验证。<u>第一版直接用上游的 BaseURL</u>，不读这个接口。
- **公告**：`GET /configs`（免登录）返回扁平字典，所有值都是字符串。
    - `public.notice` 的值是 **JSON 字符串**，要再解析一次：`{"admin":[{id, created_time, description(HTML), popup_enabled, ...}]}`。
    - 公告内容是 `admin[].description`。

## 编码计划

### 改动范围

- **新增文件**
    - `nam/internal/upstream/rightcode.go`：会话、HTTP 工具函数、Probe、Verify、Balance、Announcements、ListModels
    - `nam/internal/upstream/rightcode_tokens.go`：ListTokens、CreateToken、渠道 id 和 prefix 互查
    - `nam/internal/upstream/rightcode_pricing.go`：Pricing
    - `nam/internal/upstream/rightcode_test.go`：用 `httptest` 写的单元测试
- **修改文件**
    - `nam/internal/store/models.go`：新增常量 `UpstreamRightCode = "rightcode"`
    - `nam/internal/api/upstreams.go`：在 `upstreamTypes` 里加上 `store.UpstreamRightCode: true`
    - `web/src/api/types.ts`：在 `UpstreamType` 里加上 `'rightcode'`
    - `web/src/pages/upstreams/constants.ts`：在 `TYPE_OPTIONS` 里加上 `['rightcode', 'rightcode']`
- **可选修改**（做之前先确认）
    - `web/src/pages/upstreams/tabs/TokensTab.tsx` 里的 `canRemote` 目前只放行 newapi 和 onehub。如果要在界面上远程创建 key，要把 `rightcode` 加进去。
- **不要动的地方**
    - `adapter.go` 里的 `Adapter` 接口、`http.go` 里的现有函数。
    - `sync_checks.go` 里 `ProbeSite` 的自动探测顺序。Right Code 需要用户手动选类型，<u>不加入自动探测</u>。
    - `api/upstream_cred.go`：`access_token` 走默认分支，不用改。

### 会话与 HTTP 工具函数

现有的 `doJSON`（`http.go`）已经能处理裸 JSON：2xx 时解码，401 和 403 映射成 `ErrAuth`，出错时用 `errMessage` 取 `message`。<u>直接复用，不要再写一个 `callBare`</u>。

```go
// rightcode.go
func init() { Register(rightCodeAdapter{}) }

type rightCodeAdapter struct{}

func (rightCodeAdapter) Type() string { return store.UpstreamRightCode }

type rcSession struct {
	hc   *http.Client
	base string
	hdr  map[string]string
}

func rcOpen(s Site) (*rcSession, error) {
	if s.CredType != store.CredAccessToken {
		return nil, fmt.Errorf("cred type %q unusable for rightcode: %w", s.CredType, ErrUnsupported)
	}
	tok := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s.Cred.AccessToken), "Bearer "))
	if tok == "" {
		return nil, fmt.Errorf("user_token missing: %w", ErrAuth)
	}
	return &rcSession{hc: s.client(), base: s.BaseURL,
		hdr: map[string]string{"Authorization": "Bearer " + tok}}, nil
}

func (c *rcSession) do(ctx context.Context, method, path string, body, out any) error {
	return doJSON(ctx, c.hc, method, joinURL(c.base, path), c.hdr, body, out)
}
```

- 免登录的接口（`/configs`、`/upstreams/public`）调用 `getJSON(ctx, s.client(), joinURL(s.BaseURL, path), nil, &out)`。
- **价格字段有字符串也有数字**，需要一个辅助函数：

```go
// rcNum accepts 5, "5", "0.40000000", null, "" → *float64 (nil when absent/invalid).
func rcNum(raw json.RawMessage) *float64
```

- 实现方法：先试着解成 `float64`，失败再解成 `string` 后 `strconv.ParseFloat`。空串、`null` 和解析失败都返回 nil。

### 各方法的实现

- **Probe**（不需要凭证）
    - `GET /upstreams/public`，解析成 `{upstreams:[{id,prefix}]}`。至少要有一项，并且 `prefix` 以 `/` 开头，才算 Right Code。
    - 返回 `ProbeResult{Type: "rightcode", Evidence: "/upstreams/public 返回 N 个渠道（rightcode）"}`。
    - 其他情况返回普通 error。
- **Verify**
    - `rcOpen` 之后调用 `GET /auth/me`，解出 `{id, is_banned}`。
    - `is_banned` 为 true 时返回 `ErrAuth`，并说明"账号已封禁"。
- **Balance**
    - `GET /auth/me`，读 `balance`（可能是数字或字符串，用 `rcNum`）。
    - 返回 `Balance{USD: *v, Raw: "balance=497.478201"}`。
- **Announcements**
    - `GET /configs` 解成 `map[string]string`，取 `public.notice`。
    - 这个值要再解一次，结构是 `struct{ Admin []struct{ Description string `json:"description"` } `json:"admin"` }`。
    - 把非空的 `description` 用 `"\n\n"` 拼接，`TrimSpace` 后返回。
    - 键不存在或值为空串时，返回空的 `Announcement{}`，**不报错**。
- **ListModels**：`return listModelsOpenAI(ctx, s, key)`。

### ListTokens 与 CreateToken

- **渠道表**：写一个 `rcUpstreams(ctx, s) ([]rcUpstream, error)`。
    - 先用会话调 `/upstreams/effective`。失败且不是 `ErrAuth` 时，回退到免登录的 `/upstreams/public`。
    - `rcUpstream` 有四个字段：`ID int`、`Name string`、`Prefix string`、`Rate json.RawMessage`。
- **分组名就是 `prefix`**，例如 `/codex`。项目里的 `Token.Group`、`TokenCreate.Group`、`GroupRatio.Name` 都用它。
- **ListTokens**
    - `GET /api-key/list`，解成 `{keys:[rcKey]}`，再拿渠道表建立 id → prefix 的映射。
    - 对每个 key 生成一个 `Token`：
        - `RemoteID`：`strconv.Itoa(id)`
        - `Name`：`name`
        - `Key`：`key`（已经是完整明文，不用再补 `sk-`）
        - `Group`：`bound_upstream_id` 对应的 prefix。为 null 时用空串，找不到 id 时用 `"#<id>"`
        - `Status`：`is_active` 为真、`disabled_by_security_review` 不为 true、而且没过期，就是 1，否则是 2
        - `quota_limit` 为 null 时：`Unlimited = true`，`QuotaUSD = nil`
        - `quota_limit` 不为 null 时：`QuotaUSD` 等于剩余额度，即 `max(0, quota_limit - used_quota)`
    - 判断是否过期：`expired_at` 用 `time.ParseInLocation("2006-01-02T15:04:05", v, 东八区)` 解析，早于当前时间就算过期。东八区用 `time.FixedZone("CST", 8*3600)`，不要依赖系统的时区数据库。
- **CreateToken**
    - 在渠道表里按 `prefix == in.Group` 找 id。`in.Group` 为空或找不到时，返回 `fmt.Errorf("rightcode: 未知分组 %q", in.Group)`。
    - **不支持创建旧式 key**（不绑定渠道的 key）。
    - `POST /api-key/create`，请求体是 `{name, bound_upstream_id, allowed_models: [], allow_wallet: true, allowed_item_ids: [], quota_limit}`。
        - `quota_limit` 取 `in.QuotaUSD`，为 nil 时发 JSON `null`。
        - 新 key 的 `used_quota` 是 0，所以"总上限"就等于"剩余额度"，可以直接传。
    - 响应解成 `{id int}`，然后 `GET /api-key/{id}`，把结果转成 `Token` 返回。
    - 如果 id 为 0，就回退：拉列表，取同名、`id` 最大的那一个。都找不到时返回 error。

### Pricing

- **Groups**：从渠道表生成，`GroupRatio{Name: prefix, Ratio: rcNum(rate), Desc: name}`，按 Name 排序。
- **Models**：`GET /models/effective`，需要会话。失败且不是 `ErrAuth` 时，回退到免登录的 `/models/public`。
    - 对每个渠道头里的每个模型生成一条 `ModelPrice{Model: name, Group: prefix}`。
    - `is_available == false` 的模型**也保留**，交给上层处理。
- **取价格**：从 `effective_price_config` 取。
    - 有扁平字段（`input_price` 等）时，直接用扁平字段。
    - 没有扁平字段但有 `tiers` 时，取 `tiers[0]`。
    - 两者都没有时（例如 `{}`），回退到模型条目上的数字字段 × `effective_total_multiplier`。
- **字段映射**：
    - `input_price` → `Input`
    - `output_price` → `Output`
    - `cache_read_input_price` → `CacheRead`
    - `cache_creation_input_price` → `CacheWrite`
    - 模型条目的 `request_price` 不为 null 时 → `PerRequest`（乘 `effective_total_multiplier`）
    - `ModelRatio` 留 nil。Right Code 没有 NewAPI 那种模型倍率的概念。
- **价格里已经乘过倍率**：项目约定 `ModelPrice` 是"已经乘过分组倍率的实际价"。
    - 证据：`pricing/discount.go` 的注释写着 "group ratio already applied by the adapter"。NewAPI 适配器的 `modelPriceFor` 也是先乘 `g.Ratio` 再返回。
    - 所以用 `effective_price_config` 是对的，<u>不要再乘一次 rate</u>。

### 单元测试

- 测试文件 `rightcode_test.go` 参考 `sub2api_test.go` 的写法：用 `httptest.NewServer` 搭假站点，按路径返回固定 JSON。
- 假数据从 `temp/rightcode/raw-auth/`、`temp/rightcode/raw/` 里截取，**key 和 token 一律换成假值**（如 `sk-test`、`00000000-0000-0000-0000-000000000000`）。
- 需要覆盖的用例：
    - **Probe**：`/upstreams/public` 正常时识别成功；返回 404 或空列表时识别失败。
    - **鉴权**：请求头必须是 `Bearer <token>`；凭证里自带 `Bearer ` 前缀时不能重复添加；空 token 返回 `ErrAuth`；假站点返回 401 时得到 `ErrAuth`。
    - **Balance**：`balance` 是数字和是字符串两种情况都能解析。
    - **ListTokens**：有绑定渠道的 key、旧式 key（null）、有额度上限的 key、已过期的 key、`disabled_by_security_review` 为 true 的 key。逐个检查 Group、Status、QuotaUSD、Unlimited。
    - **CreateToken**：检查请求体字段，包括 `quota_limit` 为 null 时发出的 JSON 是 `null`；之后用 `GET /api-key/{id}` 取回 key；分组不存在时报错。
    - **Pricing**：普通模型、分档模型（只有 `tiers`）、按次计费的模型、`effective_price_config` 为 `{}` 时的回退；`/models/effective` 返回 500 时回退到 `/models/public`。
    - **Announcements**：`public.notice` 正常、缺失、为空串三种情况。
- 跑测试：在 `nam/` 目录下执行 `go test ./internal/upstream/ -run RightCode -count=1`，预计 1 分钟内完成，加上 `-timeout 120s`。
- 最后跑一遍全量：`go vet ./...`、`go test ./... -timeout 600s`。前端执行 `npm run build`（在 `web/` 目录下）。

## 验收与注意事项

### 编码 Agent 的规则

- **禁止对真实站点发写请求**。不能调用 `POST /api-key/create`、`/auth/reset-user-token`，也不能发任何 PATCH 或 DELETE。
    - 想要真机验证 CreateToken，<u>先问用户</u>，用户同意后才能调用。
- 真机只读验证可以做：`GET /auth/me`、`/api-key/list`、`/upstreams/effective`、`/models/effective`、`/configs`。token 由用户提供。
- 代码、测试、日志、提交记录里**不能出现真实的 token 或 key**。
- 不要修改 `Adapter` 接口和 `http.go` 里已有的函数。改动要小，遵守现有的代码风格（错误统一用 `fmt.Errorf("...: %w", err)` 包装）。
- 不要自动提交，等用户确认后再提交。

### 验收标准

- `go vet ./...` 和 `go test ./...` 全部通过，新增的测试覆盖了上一节列出的用例。
- `npm run build` 通过，前端新增上游时能选到 `rightcode`，凭证方式选 Access Token。
- 用户手动验证：在界面上新增一个 Right Code 上游，BaseURL 填 `https://rightapi.io`，Access Token 填 user_token。然后依次检查：
    - 验证凭证成功
    - 余额和网页上显示的一致
    - 令牌列表和网页上的 API Key 一致
    - 分组列表里有 `/codex`、`/claude` 等 13 个渠道
    - 价格和网页"模型"页的实际价一致

### 待确认的问题

- **`/api-key/create` 的响应**：没有实测过，不确定是否包含完整 key。代码已经按"只有 id"的情况兜底。
- **token 轮换**：账号开着 30 天自动轮换。轮换后会收到 401，需要用户手动更新凭证。也可以在站点设置里关掉轮换。
- **中转域名**：`/account/relay-domains` 返回的 `https://api.site` 是否可用未知。第一版的渠道 BaseURL 直接用 `rightapi.io`。
- **deepseek 时段计价**：effective 和 public 的价格差了一半，要上线后对账确认。
- **旧式 key**（不绑定渠道）：Group 为空串，上层同步时怎么处理，需要用户决定。
- **订阅额度**：第一版不计入余额。如果用户有订阅，再考虑接入 `/subscriptions/list` 里的 `remaining_quota`。
