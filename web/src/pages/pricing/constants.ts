import type { PriceCounts, PriceRow } from '@/api/types';
import { visiblePriceStatus, type VisiblePriceStatus } from '@/lib/labels';

export type PriceTab = 'all' | VisiblePriceStatus;

export const STATUS_TABS: Array<{ value: PriceTab; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'normal', label: '正常' },
  { value: 'no_price', label: '缺价' },
  { value: 'per_request', label: '按次计费' },
];

/** 后端的「偏差」数量并入「正常」。 */
export function tabCounts(counts: PriceCounts): Record<PriceTab, number> {
  return {
    all: counts.all,
    normal: counts.normal + counts.deviation,
    no_price: counts.no_price,
    per_request: counts.per_request,
  };
}

/** 按页签过滤。「正常」页签要把后端标成偏差的行也包含进来，所以在前端筛。 */
export function matchesTab(row: PriceRow, tab: PriceTab): boolean {
  return tab === 'all' || visiblePriceStatus(row.status) === tab;
}

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
