import type { LayoutDoc } from '@/lib/layout/store';

export interface Envelope<T> {
  success: boolean;
  message: string;
  data: T;
}

export type Mode = 'embedded' | 'standalone';

export interface Identity {
  user_id: number;
  username: string;
  role: number;
}

export interface Session {
  mode: Mode;
  identity: Identity;
  version: string;
}

export interface BackendInfo {
  mode: string;
  version: string;
  address: string;
  patch_p1: boolean;
  quota_per_unit: number;
  last_request_at: number;
  last_error: string;
}

export type UpstreamType = 'newapi' | 'sk-key' | 'sub2api' | 'onehub' | 'official' | 'generic' | 'rightcode';
export type CredType = '' | 'access_token' | 'password' | 'sk_key' | 'cookie' | 'sub2api_session';

export interface Credential {
  access_token?: string;
  username?: string;
  password?: string;
  key?: string;
  cookie?: string;
  refresh_token?: string;
}

export interface UpstreamInput {
  name?: string;
  type?: UpstreamType;
  base_url?: string;
  cred_type?: CredType;
  credential?: Credential;
  platform_user_id?: number;
  currency_rate?: number;
  tags?: string;
  remark?: string;
  adapter_config?: string;
}

export interface Upstream {
  id: number;
  name: string;
  type: UpstreamType;
  base_url: string;
  cred_type: CredType;
  has_credential: boolean;
  platform_user_id: number;
  cred_updated_at: number;
  cred_expires_at: number;
  currency_rate: number;
  tags: string;
  remark: string;
  adapter_config: string;
  balance_usd: number | null;
  balance_cny: number | null;
  balance_at: number;
  last_sync_at: number;
  last_sync_error: string;
  token_count: number;
  channel_count: number;
  created_at: string;
  updated_at: string;
}

export type JobName =
  | 'balance'
  | 'ratio'
  | 'price_book'
  | 'announcements'
  | 'pull_logs'
  | 'pricing'
  | 'consistency'
  | 'cleanup'
  | 'backup';

export interface JobStatus {
  name: JobName;
  title: string;
  interval_s: number;
  last_start: number;
  last_duration_ms: number;
  last_result: '' | 'ok' | 'fail';
  last_message: string;
  running: boolean;
  queued: boolean;
  next_at: number;
}

/**
 * 设置项：键值都是字符串。本项目用到的键见各设置分区；
 * usd_cny_rate：美元参考汇率（元/$），正数且 ≤ 100，默认 7.2，用于把成本换算成原厂价的百分比。
 */
export type Settings = Record<string, string>;

export type AuditKind = 'channel' | 'price' | 'setting' | 'upstream' | 'cache_test';
export type AuditTrigger = 'auto' | 'manual';
export type AuditResult = 'ok' | 'fail' | 'skipped';

export interface AuditLog {
  id: number;
  at: number;
  kind: AuditKind;
  target: string;
  action: string;
  summary: string;
  before: string;
  after: string;
  request: string;
  response: string;
  trigger: AuditTrigger;
  result: AuditResult;
  error: string;
  operator: string;
  rollback_of: number | null;
}

export interface AuditQuery {
  kind?: string;
  trigger?: string;
  result?: string;
  from?: number;
  to?: number;
  q?: string;
  limit?: number;
  offset?: number;
}

export interface Page<T> {
  total: number;
  items: T[];
}

export interface LoginFailure {
  failures?: number;
  locked_until?: number;
}

export * from './types-channels';
export * from './types-pricing';
export * from './types-stats';

/** GET /ui/layout：当前用户保存的界面布局；没有保存过时 layout 为 null、saved_at 为 0。 */
export interface UiLayoutResponse {
  layout: LayoutDoc | null;
  saved_at: number;
}

export interface PasswordInput {
  old_password: string;
  new_password: string;
  confirm_password: string;
}
