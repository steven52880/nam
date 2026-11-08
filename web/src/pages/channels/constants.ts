import type { SyncState } from '@/api/types';

export const STATUS_LABELS: Record<number, string> = { 1: '启用', 2: '手动禁用', 3: '自动禁用' };

export const SYNC_LABELS: Record<SyncState, string> = {
  synced: '已同步',
  draft: '草稿',
  draft_create: '草稿·新建',
  draft_delete: '草稿·删除',
  drift: '漂移',
  error: '错误',
};

export type GroupMode = 'upstream' | 'vendor' | 'tag' | 'none';

export const GROUP_MODE_OPTIONS: Array<[GroupMode, string]> = [
  ['upstream', '按上游'],
  ['vendor', '按厂商'],
  ['tag', '按标签'],
  ['none', '不分组'],
];

export const STATUS_FILTER_OPTIONS: Array<[string, string]> = [
  ['1', '启用'],
  ['2', '手动禁用'],
  ['3', '自动禁用'],
];

export const SYNC_FILTER_OPTIONS: Array<[SyncState, string]> = [
  ['synced', '已同步'],
  ['draft', '草稿'],
  ['drift', '漂移'],
  ['error', '错误'],
];

/** 列 id 对应的草稿 / 漂移字段名，用于高亮。 */
export const COLUMN_FIELDS: Record<string, string[]> = {
  status: ['status'],
  name: ['name'],
  vendor: ['vendor', 'group'],
  models: ['models'],
  discount: ['discount', 'discount_source'],
  priority: ['priority'],
  weight: ['weight'],
  tag: ['tag'],
};

/** 允许复制粘贴的列（顺序即复制时的列顺序）。 */
export const COPYABLE_COLUMNS = ['status', 'discount', 'priority', 'weight', 'tag', 'models'];

export const ALL_FILTER = 'all';
