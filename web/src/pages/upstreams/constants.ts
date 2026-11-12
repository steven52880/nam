import type { CredType, Upstream, UpstreamType } from '@/api/types';

export const TYPE_OPTIONS: Array<[UpstreamType, string]> = [
  ['newapi', 'newapi'],
  ['sk-key', 'sk-key'],
  ['sub2api', 'sub2api'],
  ['onehub', 'onehub'],
  ['official', 'official'],
  ['generic', 'generic'],
];

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

/** 按上游类型过滤可选的凭证方式：cookie 只给 newapi / onehub，sub2api_session 只给 sub2api（排在最前）。 */
export function credOptionsFor(type: UpstreamType): Array<[CredType, string]> {
  const label = (c: CredType) => CRED_OPTIONS.find(([v]) => v === c)?.[1] ?? c;
  const common = CRED_OPTIONS.filter(([v]) => v !== 'cookie' && v !== 'sub2api_session');
  if (type === 'newapi' || type === 'onehub') {
    const [first, ...rest] = common;
    return first ? [first, ['cookie', label('cookie')], ...rest] : common;
  }
  if (type === 'sub2api') return [['sub2api_session', label('sub2api_session')], ...common];
  return common;
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
