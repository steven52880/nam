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
  ['', '暂不填写'],
];

export const CRED_LABELS: Record<string, string> = {
  access_token: 'Access Token',
  password: '账号密码',
  sk_key: 'sk-key',
};

export function splitTags(tags: string): string[] {
  return tags
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean);
}

export function allTags(list: Upstream[]): string[] {
  return [...new Set(list.flatMap((u) => splitTags(u.tags)))].sort();
}
