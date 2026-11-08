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
  sync_state: SyncState;
  dirty_fields: string[];
  drift_fields: string[];
  draft_id?: number;
  draft_op?: DraftOp;
  draft_error?: string;
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
  models: string[];
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
  pattern: string;
  order: number;
  channel_type: 58 | 24;
}

export interface VendorInput {
  name: string;
  group_name: string;
  models: string;
  pattern?: string;
  channel_type: 58 | 24;
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

export interface SyncResult {
  what: string;
  ok: boolean;
  count: number;
  error?: string;
}

