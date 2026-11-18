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
  discount?: number;
  discount_source?: DiscountSource;
}

export interface ChannelChange {
  channel_id: number;
  fields: ChannelFields;
  ext: ChannelExtFields;
}

export type DiscountSource = 'auto' | 'manual';
export type SyncState = 'synced' | 'draft' | 'draft_create' | 'draft_delete' | 'drift' | 'error';
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
  discount: number;
  discount_source: DiscountSource;
  currency_rate: number | null;
  channel_cost: number | null;
  balance_usd: number | null;
  balance_cny: number | null;
  /** 单次成本（元）：标准请求 1000 输入其中 200 缓存 + 500 输出。 */
  unit_cost: number | null;
  unit_cost_model: string;
  sync_state: SyncState;
  dirty_fields: string[];
  drift_fields: string[];
  draft_id?: number;
  draft_op?: DraftOp;
  draft_error?: string;
}

/** 某个模型在各渠道上的成本（GET /channels/costs?model=）。行集合与渠道列表相同（含负数 id 的草稿新建渠道）。 */
export interface ModelCost {
  channel_id: number;
  /** 渠道是否包含该模型；为 false 时其余字段都是 null。 */
  has_model: boolean;
  /** 该模型的生效折扣（含模型级折扣覆盖）。 */
  discount: number | null;
  /** 折扣 × 上游货币汇率；没有上游时为 null。 */
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

export interface DriftField {
  field: string;
  old: unknown;
  new: unknown;
}

export interface DriftDetail {
  channel_id: number;
  name: string;
  fields: DriftField[];
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
  renamed_drafts: number;
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

