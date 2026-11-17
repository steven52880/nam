import type { CredType, Upstream, UpstreamType } from '@/api/types';

export const CRED_OPTIONS: Array<[CredType, string]> = [
  ['access_token', 'Access Token + 用户 ID'],
  ['password', '账号 + 密码'],
  ['sk_key', '仅 sk-key'],
  ['cookie', '会话 Cookie + 用户 ID'],
  ['sub2api_session', '登录令牌（自动续期）'],
  ['', '暂不填写'],
];

export const CRED_LABELS: Record<string, string> = {
  access_token: 'Access Token',
  password: '账号密码',
  sk_key: 'sk-key',
  cookie: '会话 Cookie',
  sub2api_session: '登录令牌（自动续期）',
};

/** 是否需要填平台用户 ID：access_token / cookie 需要，Right Code 只用 token 做 Bearer，不需要。 */
export function needsUserId(type: UpstreamType, credType: CredType): boolean {
  if (type === 'rightcode') return false;
  return credType === 'access_token' || credType === 'cookie';
}

/** 凭证方式的下拉标签：Right Code 的 access_token 实际填的是 user_token，没有用户 ID。 */
export function credOptionLabel(type: UpstreamType, credType: CredType, fallback: string): string {
  if (type === 'rightcode' && credType === 'access_token') return 'Access Token（user_token）';
  return fallback;
}

/** 按上游类型过滤可选的凭证方式：cookie 只给 newapi / onehub，sub2api_session 只给 sub2api（排在最前）。 */
export function credOptionsFor(type: UpstreamType): Array<[CredType, string]> {
  const label = (c: CredType) => CRED_OPTIONS.find(([v]) => v === c)?.[1] ?? c;
  const common = CRED_OPTIONS.filter(([v]) => v !== 'cookie' && v !== 'sub2api_session');
  // Right Code 登录需要腾讯验证码，只能手填浏览器里的 user_token。
  if (type === 'rightcode') {
    return common
      .filter(([v]) => v === 'access_token' || v === '')
      .map(([v, l]) => [v, credOptionLabel(type, v, l)] as [CredType, string]);
  }
  if (type === 'newapi' || type === 'onehub') {
    const [first, ...rest] = common;
    return first ? [first, ['cookie', label('cookie')], ...rest] : common;
  }
  if (type === 'sub2api') return [['sub2api_session', label('sub2api_session')], ...common];
  return common;
}

/** 在 credOptionsFor 基础上保留当前（可能是历史遗留的）凭证方式，避免选中值在下拉里消失。 */
export function credOptionsWithCurrent(type: UpstreamType, current: CredType): Array<[CredType, string]> {
  const options = credOptionsFor(type);
  if (options.some(([v]) => v === current)) return options;
  const legacy = CRED_OPTIONS.find(([v]) => v === current);
  return legacy ? [...options, legacy] : options;
}

export function allowsCredType(type: UpstreamType, credType: CredType): boolean {
  return credOptionsFor(type).some(([v]) => v === credType);
}

/** 新增时的默认凭证方式：sub2api 默认登录令牌，其余沿用 access_token。 */
export function defaultCredType(type: UpstreamType): CredType {
  return type === 'sub2api' ? 'sub2api_session' : 'access_token';
}

export function splitTags(tags: string): string[] {
  return tags
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean);
}

export function allTags(list: Upstream[]): string[] {
  return [...new Set(list.flatMap((u) => splitTags(u.tags)))].sort();
}
