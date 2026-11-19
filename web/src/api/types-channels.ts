import type { Credential, CredType, UpstreamType } from './types';

/* ---------- M2：渠道 / 厂商 / 令牌 ---------- */

export interface ChannelFields {
  name?: string;
  base_url?: string;
  models?: string[];
  group?: string;
  priority?: number;
  weight?: number;
  tag?: string;
  status?: number;
  test_model?: string;
  type?: number;
}

export interface ChannelExtFields {
  upstream_id?: number;
  token_id?: number;
  vendor?: string;
  /** 手填倍率；和 discount_source: 'manual' 等价。 */
  discount?: number;
  /** 'auto'（不带 discount）= 清掉手填，回到自动。 */
  discount_source?: DiscountSource;
}

export interface ChannelChange {
  channel_id: number;
  fields: ChannelFields;
  ext: ChannelExtFields;
}

/** 渠道级倍率来源：manual 手填 / auto 按上游逐模型价推算 / group 上游分组倍率 / none 算不出。 */
export type DiscountSource = 'manual' | 'auto' | 'group' | 'none';
export type SyncState = 'synced' | 'draft' | 'draft_create' | 'draft_delete' | 'error';
export type DraftOp = 'create' | 'update' | 'delete';

export interface ChannelRow {
  id: number;
  type: number;
  name: string;
  status: number;
  base_url: string;
  models: string[];
  group: string;
  priority: number;
  weight: number;
  tag: string;
  test_model: string;
  response_time: number;
  test_time: number;
  used_quota: number;
  managed: boolean;
  upstream_id: number | null;
  upstream_name: string;
  token_id: number | null;
  token_name: string;
  vendor: string;
  /** 上游分组倍率：上游对该分组请求的实际扣费是上游美元标价的多少倍（无单位，显示小数）；算不出时为 null。 */
  discount: number | null;
  discount_source: DiscountSource;
  /** discount_source 为 none 时说明为什么算不出，其他来源为空串。 */
  discount_reason: string;
  /** 令牌所在分组在上游的分组倍率；没有时为 null。 */
  upstream_group_ratio: number | null;
  /** 上游货币汇率（元/$）：在上游充值 1 美元额度需要的人民币。 */
  currency_rate: number | null;
  /** 上游渠道售价 = 分组倍率 × 上游货币汇率 ÷ 美元参考汇率（后端已除），值是原厂价的倍数，前端显示成百分比。 */
  channel_cost: number | null;
  balance_usd: number | null;
  balance_cny: number | null;
  /** 单次成本（元）：标准请求 1000 输入其中 200 缓存 + 500 输出。 */
  unit_cost: number | null;
  unit_cost_model: string;
  sync_state: SyncState;
  dirty_fields: string[];
  draft_id?: number;
  draft_op?: DraftOp;
  draft_error?: string;
}

/** 某个模型在各渠道上的成本（GET /channels/costs?model=）。行集合与渠道列表相同（含负数 id 的草稿新建渠道）。 */
export interface ModelCost {
  channel_id: number;
  /** 渠道是否包含该模型；为 false 时其余字段都是 null。 */
  has_model: boolean;
  /** 该模型的生效上游分组倍率（含模型级倍率覆盖）。 */
  discount: number | null;
  /** 上游渠道售价（原厂价的倍数，后端已除以美元参考汇率）；没有上游时为 null。 */
  channel_cost: number | null;
  /** 该模型的单次成本（元），标准请求同 ChannelRow.unit_cost。 */
  unit_cost: number | null;
}

export interface ModelCostList {
  model: string;
  items: ModelCost[];
}

export interface ChannelList {
  items: ChannelRow[];
  draft_count: number;
}

export interface ValidationIssue {
  channel_id: number;
  name: string;
  field: string;
  message: string;
}

export interface IssuesPayload {
  issues: ValidationIssue[];
}

/** GET /channels/issues：对所有渠道（含草稿）跑规则检查得到的问题。 */
export interface ChannelIssueList {
  items: ValidationIssue[];
}

export interface PlanChange {
  field: string;
  old: unknown;
  new: unknown;
  newapi: boolean;
}

export interface PlanRequest {
  method: string;
  path: string;
  body?: unknown;
}

export interface PlanItem {
  draft_id: number;
  channel_id: number;
  op: DraftOp;
  name: string;
  changes: PlanChange[];
  request?: PlanRequest;
}

export interface ApplyResult {
  draft_id: number;
  channel_id: number;
  op: DraftOp;
  name: string;
  ok: boolean;
  error?: string;
}

export interface ApplyResponse {
  results: ApplyResult[];
  success: number;
  failed: number;
}

export interface BatchRequest {
  upstream_id: number;
  token_ids: number[];
  /** 键是令牌 id 的字符串。 */
  token_models: Record<string, string[]>;
  priority: number;
  weight: number;
  tag: string;
  test_model?: Record<string, string>;
  discount?: Record<string, number>;
}

export interface BatchSpec {
  fields: ChannelFields;
  ext: ChannelExtFields;
}

export interface BatchPreviewItem {
  fields: ChannelFields;
  ext: ChannelExtFields;
  /** 预览里现算的渠道倍率；后端没给（undefined）= 创建后自动；null = 无法推算。 */
  discount?: number | null;
  discount_source?: DiscountSource;
  discount_reason?: string;
  name: string;
  token_name: string;
  vendor: string;
  group: string;
  channel_type: number;
  duplicate: boolean;
}

export interface BatchPreview {
  items: BatchPreviewItem[];
  unassigned: string[];
  /** 键是令牌 id 的字符串。 */
  unassigned_by_token: Record<string, string[]>;
  warnings: string[];
}

export interface MatchTokenItem {
  channel_id: number;
  token_id: number | null;
  token_name: string;
  created: boolean;
  needs_manual: boolean;
  message: string;
}

export interface MatchTokensRequest {
  upstream_id: number;
  channel_ids: number[];
}

export interface MatchTokensResponse {
  items: MatchTokenItem[];
}

export interface CheckResult {
  name: string;
  ok: boolean;
  skipped: boolean;
  message: string;
  ms: number;
}

export interface ProbeResult {
  adapter?: string;
  type: UpstreamType;
  evidence: string;
  version?: string;
  login_captcha?: boolean;
}

export interface UpstreamTestInput {
  type: UpstreamType;
  base_url: string;
  cred_type: CredType;
  credential: Credential;
  platform_user_id: number;
  adapter_config: string;
}

export interface Vendor {
  id: number;
  name: string;
  group_name: string;
  models: string;
  /** 模型名前缀：不区分大小写，最长前缀优先。 */
  prefixes: string[];
  order: number;
  channel_type: 58 | 24;
}

/** 编辑厂商的返回：厂商本身 + 本次生成的草稿数。 */
export interface VendorUpdateResult extends Vendor {
  group_drafts: number;
}

export interface VendorInput {
  name: string;
  group_name: string;
  models: string[];
  prefixes: string[];
  channel_type: 58 | 24;
}

/** 厂商匹配方式：explicit = 显式模型，prefix = 最长前缀，'' = 未归属。 */
export type VendorMatchBy = 'explicit' | 'prefix' | '';

export interface VendorMatchItem {
  model: string;
  vendor: string;
  by: VendorMatchBy;
  prefix: string;
}

export interface VendorMatchResponse {
  items: VendorMatchItem[];
}

export interface UpstreamToken {
  id: number;
  upstream_id: number;
  remote_id: string;
  name: string;
  group: string;
  key_hint: string;
  quota_usd: number | null;
  unlimited: boolean;
  status: number;
  manual: boolean;
  updated_at: number;
  channel_count?: number;
}

export interface TokenInput {
  name: string;
  group: string;
  key?: string;
  quota_usd?: number | null;
}

export interface RemoteTokenInput {
  name: string;
  group: string;
  quota_usd: number | null;
}

export interface UpstreamGroup {
  name: string;
  ratio: number;
  desc: string;
  updated_at: number;
}

export interface UpstreamModel {
  id: number;
  upstream_id: number;
  group: string;
  model: string;
  model_ratio: number | null;
  input: number | null;
  output: number | null;
  cache_read: number | null;
  cache_write: number | null;
  per_request: number | null;
  updated_at: number;
}

export interface ChannelTestResult {
  success: boolean;
  message: string;
  duration: number;
}

/* ---------- M3.7：手动缓存率测试 ---------- */

export type CacheTestProtocol = 'anthropic' | 'openai';
export type CacheTestStatus = 'running' | 'ok' | 'fail';
/** direct：nam 直连上游（默认）；newapi：经 NewAPI 中转（预留，仅 API 可达）。 */
export type CacheTestMode = 'direct' | 'newapi';

export interface CacheTestStartInput {
  model: string;
  rounds?: number;
  protocol?: CacheTestProtocol;
  mode?: CacheTestMode;
}

export interface CacheTest {
  id: number;
  channel_id: number;
  model: string;
  protocol: CacheTestProtocol;
  mode: CacheTestMode;
  status: CacheTestStatus;
  rounds: number;
  input_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  rate: number | null;
  /** JSON 字符串：CacheTestRound[]。 */
  per_round: string;
  error: string;
  /** Unix 秒。 */
  started_at: number;
  duration_ms: number;
}

export interface CacheTestRound {
  round: number;
  input: number;
  cache_read: number;
  cache_write: number;
  output: number;
  duration_ms: number;
}

export interface CacheTestList {
  items: CacheTest[];
}

export interface SyncResult {
  what: string;
  ok: boolean;
  count: number;
  error?: string;
}

