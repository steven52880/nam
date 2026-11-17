export type GroupMode = 'upstream' | 'vendor' | 'tag' | 'none';

export const GROUP_MODE_OPTIONS: Array<[GroupMode, string]> = [
  ['upstream', '按上游'],
  ['vendor', '按厂商'],
  ['tag', '按标签'],
  ['none', '不分组'],
];

/** 列 id 对应的草稿 / 漂移字段名，用于高亮。 */
export const COLUMN_FIELDS: Record<string, string[]> = {
  status: ['status'],
  name: ['name'],
  upstream: ['upstream_id'],
  token: ['token_id'],
  vendor: ['vendor'],
  group: ['group'],
  models: ['models'],
  discount: ['discount', 'discount_source'],
  priority: ['priority'],
  weight: ['weight'],
  tag: ['tag'],
};

/** 允许复制粘贴的列（顺序即复制时的列顺序）。 */
export const COPYABLE_COLUMNS = ['status', 'discount', 'priority', 'weight', 'tag', 'models'];

export const ALL_FILTER = 'all';
