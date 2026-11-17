import { useMemo } from 'react';
import { useChannelStats as useChannelStatsQuery } from '@/api/queries';
import type { ChannelRow, ChannelStatsItem, StatsWindow } from '@/api/types';
import { formatPercent } from '@/lib/format';

const NO_STATS = new Map<number, ChannelStatsItem>();

/** 渠道统计。接口出错或尚未实现时返回空表，界面显示「—」，不弹提示。 */
export function useChannelStats(statsWindow: StatsWindow) {
  const query = useChannelStatsQuery(statsWindow);
  const stats = useMemo(
    () => (query.data ? new Map(query.data.items.map((item) => [item.channel_id, item])) : NO_STATS),
    [query.data],
  );
  return { stats, errorLogHint: query.data?.error_log_hint ?? false };
}

/** 组标题行摘要：渠道数、启用数、平均成功率（按请求数加权）、成本合计。 */
export function summarizeGroup(rows: ChannelRow[], stats: Map<number, ChannelStatsItem>): string {
  let requests = 0;
  let success = 0;
  let cost = 0;
  for (const row of rows) {
    const item = stats.get(row.id);
    if (!item) continue;
    requests += item.requests;
    success += item.success;
    cost += item.cost_cny;
  }
  const enabled = rows.filter((r) => r.status === 1).length;
  const rate = requests > 0 ? formatPercent(success / requests) : '—';
  const costText = stats.size > 0 ? `¥${cost.toFixed(2)}` : '—';
  return `${rows.length} 个渠道 · 启用 ${enabled} · 平均成功率 ${rate} · 成本 ${costText}`;
}
