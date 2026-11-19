/**
 * 全部布局槽位名的登记表。写错会编译失败。
 * 第三批一次登记完，之后各分支不需要再改这个文件。
 */
export const TABLE_LAYOUT_IDS = [
  'channels.main', 'channels.models', 'channels.cache-rounds',
  'upstreams.list', 'upstream.tokens', 'upstream.groups', 'upstream.prices', 'upstream.derive',
  'pricing.list', 'pricing.channels', 'pricing.vendors', 'audit.list', 'overview.jobs',
  'stats.leaderboard', 'stats.hourly', 'settings.vendors',
  'wizard.upstreams', 'wizard.tokens', 'wizard.preview', 'demo.grid',
] as const;

export type TableLayoutId = (typeof TABLE_LAYOUT_IDS)[number];
export type PageSlotId = 'page:channels';
export type LayoutSlotId = `table:${TableLayoutId}` | PageSlotId;

/** 表格布局槽位名。 */
export function tableSlotId(id: TableLayoutId): `table:${TableLayoutId}` {
  return `table:${id}`;
}
