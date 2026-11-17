import type { PriceStatus } from '@/api/types';

export const STATUS_TABS: Array<{ value: 'all' | PriceStatus; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'normal', label: '正常' },
  { value: 'deviation', label: '偏差' },
  { value: 'no_price', label: '缺价' },
  { value: 'per_request', label: '按次计费' },
];

/** 偏差超过 5% 标红。 */
export const DEVIATION_THRESHOLD = 0.05;

export const SOURCE_LABELS: Record<string, string> = {
  override: '手动覆盖',
  manual: '手动覆盖',
  litellm: '价格库',
  models_dev: '价格库',
  'models.dev': '价格库',
};

export function sourceLabel(source: string): string {
  if (!source) return '—';
  return SOURCE_LABELS[source] ?? source;
}

export const ALL_VENDORS = '__all__';
