import { useMemo } from 'react';
import { useChannelStats as useChannelStatsQuery } from '@/api/queries';
import type { ChannelStatsItem, StatsWindow } from '@/api/types';

const NO_STATS = new Map<number, ChannelStatsItem>();

/** 渠道统计。model 为 null 时是渠道汇总，否则只统计该模型。接口出错或尚未实现时返回空表，界面显示「—」，不弹提示。 */
export function useChannelStats(statsWindow: StatsWindow, model: string | null) {
  const query = useChannelStatsQuery(statsWindow, model);
  const stats = useMemo(
    () => (query.data ? new Map(query.data.items.map((item) => [item.channel_id, item])) : NO_STATS),
    [query.data],
  );
  return { stats, errorLogHint: query.data?.error_log_hint ?? false };
}
