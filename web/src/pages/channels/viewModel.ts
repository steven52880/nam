import type { ChannelRow, ChannelStatsItem } from '@/api/types';
import { costRangeOf, type CostRange } from '@/components/cells/costColor';
import { formatPercent } from '@/lib/format';
import { filterChannels, type ChannelFilters } from './filter';

/** 表格行：渠道 + 渠道汇总统计（没有统计、草稿新建渠道为 null）。 */
export type ChannelViewRow = ChannelRow & { stat: ChannelStatsItem | null };

export interface ChannelRanges {
  channelCost: CostRange | null;
  unitCost: CostRange | null;
  effectiveCost: CostRange | null;
  stdCost: CostRange | null;
}

/** 筛选 → 合并统计 → 按可见行计算成本颜色范围。 */
export function buildChannelView(input: {
  rows: ChannelRow[];
  stats: ReadonlyMap<number, ChannelStatsItem>;
  filters: ChannelFilters;
}): { rows: ChannelViewRow[]; ranges: ChannelRanges } {
  const rows = filterChannels(input.rows, input.filters).map<ChannelViewRow>((r) => ({
    ...r,
    stat: input.stats.get(r.id) ?? null,
  }));
  return {
    rows,
    ranges: {
      channelCost: costRangeOf(rows.map((r) => r.channel_cost)),
      unitCost: costRangeOf(rows.map((r) => r.unit_cost)),
      effectiveCost: costRangeOf(rows.map((r) => r.stat?.effective_cost_price ?? null)),
      stdCost: costRangeOf(rows.map((r) => r.stat?.std_cost_price ?? null)),
    },
  };
}

/** 组标题行摘要：渠道数、启用数、平均成功率（按请求数加权）、成本合计。hasStats = 统计接口有数据。 */
export function summarizeGroup(rows: ChannelViewRow[], hasStats: boolean): string {
  let requests = 0;
  let success = 0;
  let cost = 0;
  for (const row of rows) {
    const item = row.stat;
    if (!item) continue;
    requests += item.requests;
    success += item.success;
    cost += item.cost_cny;
  }
  const enabled = rows.filter((r) => r.status === 1).length;
  const rate = requests > 0 ? formatPercent(success / requests) : '—';
  const costText = hasStats ? `¥${cost.toFixed(2)}` : '—';
  return `${rows.length} 个渠道 · 启用 ${enabled} · 平均成功率 ${rate} · 成本 ${costText}`;
}
