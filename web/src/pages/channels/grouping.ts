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

export interface GroupOrderContext {
  /** useVendors() 的名称顺序（设置页拖动排序的结果）。 */
  vendorOrder: string[];
  /** 槽位里保存的上游 / 标签组顺序（上游用 upstream_id 的字符串）。 */
  savedOrder: string[];
  /** 组键 → 显示名；没有保存顺序的组按它排序。 */
  labelOf: (key: string) => string;
}

/**
 * 组顺序：已有顺序（厂商用 vendorOrder，上游 / 标签用 savedOrder）的按顺序排在前面，
 * 不在里面的按显示名排在后面，兜底组 '' 永远最后。不分组时保持原样。
 */
export function sortGroupKeys(mode: GroupMode, keys: string[], ctx: GroupOrderContext): string[] {
  if (mode === 'none') return keys.slice();
  const known = new Map<string, number>();
  (mode === 'vendor' ? ctx.vendorOrder : ctx.savedOrder).forEach((k, i) => {
    if (!known.has(k)) known.set(k, i);
  });
  return keys.slice().sort((a, b) => {
    if (a === b) return 0;
    if (a === '') return 1;
    if (b === '') return -1;
    const ia = known.get(a);
    const ib = known.get(b);
    if (ia !== undefined && ib !== undefined) return ia - ib;
    if (ia !== undefined) return -1;
    if (ib !== undefined) return 1;
    const byName = ctx.labelOf(a).localeCompare(ctx.labelOf(b), 'zh-CN');
    return byName !== 0 ? byName : a < b ? -1 : 1;
  });
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
