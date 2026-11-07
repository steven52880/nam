import type { Envelope } from './types';

export class ApiError extends Error {
  readonly status: number;
  readonly data: unknown;

  constructor(status: number, message: string, data: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.data = data;
  }
}

type Handler = () => void;
let onUnauthorized: Handler | null = null;
let onForbidden: Handler | null = null;

export function setAuthHandlers(handlers: { unauthorized: Handler; forbidden: Handler }): void {
  onUnauthorized = handlers.unauthorized;
  onForbidden = handlers.forbidden;
}

/** 记录最近一次请求/错误，供连接状态弹窗展示。 */
export const lastCall: { at: number; error: string } = { at: 0, error: '' };

function readUserId(): string | null {
  try {
    const raw = localStorage.getItem('user');
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed === 'object' && parsed !== null && 'id' in parsed) {
      const id = (parsed as { id: unknown }).id;
      if (typeof id === 'number' || typeof id === 'string') return String(id);
    }
  } catch {
    return null;
  }
  return null;
}

function isEnvelope(v: unknown): v is Envelope<unknown> {
  return typeof v === 'object' && v !== null && 'success' in v;
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  body?: unknown;
  query?: Record<string, string | number | boolean | undefined>;
  /** 为 true 时 401/403 不触发全局跳转（登录等场景）。 */
  silent?: boolean;
}

function buildUrl(path: string, query?: RequestOptions['query']): string {
  const url = `/api/nam${path}`;
  if (!query) return url;
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined && v !== '') sp.set(k, String(v));
  }
  const qs = sp.toString();
  return qs ? `${url}?${qs}` : url;
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
  const uid = readUserId();
  if (uid) headers['New-Api-User'] = uid;

  lastCall.at = Math.floor(Date.now() / 1000);
  let res: Response;
  try {
    res = await fetch(buildUrl(path, opts.query), {
      method: opts.method ?? 'GET',
      credentials: 'include',
      headers,
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    });
  } catch (e) {
    const msg = e instanceof Error ? e.message : '网络错误';
    lastCall.error = msg;
    throw new ApiError(0, `网络错误：${msg}`, null);
  }

  let payload: unknown = null;
  try {
    payload = await res.json();
  } catch {
    payload = null;
  }
  const env = isEnvelope(payload) ? payload : null;
  const message = env?.message || res.statusText || '请求失败';

  if (!res.ok || (env && !env.success)) {
    lastCall.error = `${res.status} ${message}`;
    if (!opts.silent) {
      if (res.status === 401) onUnauthorized?.();
      else if (res.status === 403) onForbidden?.();
    }
    throw new ApiError(res.status, message, env?.data ?? null);
  }
  lastCall.error = '';
  return (env ? env.data : null) as T;
}
