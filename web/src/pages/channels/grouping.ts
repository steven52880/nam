import type { ChannelRow } from '@/api/types';
import type { GroupMode } from './constants';

export interface GroupLookups {
  /** upstream_id 的字符串 → 上游名。 */
  upstreamNames: Map<string, string>;
}

/** 组键：上游用 upstream_id 的字符串；厂商、标签取原值；没有则为 ''（兜底组）。 */
export function groupKeyOf(mode: GroupMode): ((row: ChannelRow) => string) | undefined {
  switch (mode) {
    case 'upstream':
      return (r) => (r.upstream_id === null ? '' : String(r.upstream_id));
    case 'vendor':
      return (r) => r.vendor || '';
    case 'tag':
      return (r) => r.tag || '';
    case 'none':
      return undefined;
  }
}

/** 从行里建 upstream_id → upstream_name 的映射（上游改名后跟着变，组键不变）。 */
export function buildGroupLookups(rows: ChannelRow[]): GroupLookups {
  const upstreamNames = new Map<string, string>();
  for (const r of rows) {
    if (r.upstream_id !== null && r.upstream_name) upstreamNames.set(String(r.upstream_id), r.upstream_name);
  }
  return { upstreamNames };
}

/** 组标题文字；'' 是兜底组。 */
export function groupLabelOf(mode: GroupMode, key: string, lookups: GroupLookups): string {
  switch (mode) {
    case 'upstream':
      return key === '' ? '未托管' : (lookups.upstreamNames.get(key) ?? `上游 #${key}`);
    case 'vendor':
      return key === '' ? '无厂商' : key;
    case 'tag':
      return key === '' ? '无标签' : key;
    case 'none':
      return key;
  }
}
