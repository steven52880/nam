import { useChannelModels, useChannelModelStats } from '@/api/queries';
import { useMemo } from 'react';
import type { ChannelModels, ChannelRow, ModelRow, StatsWindow } from '@/api/types';
import { DataTable } from '@/components/table';
import { Badge } from '@/components/ui/misc';
import { formatNumber } from '@/lib/format';
import { CacheOverridePopover } from './ModelDiscountControls';
import { modelRowId, useModelDetailColumns } from './modelDetailColumns';

const NO_MODELS: ModelRow[] = [];

function Header({ detail }: { detail: ChannelModels }) {
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-0.5 border-b bg-muted/50 px-3 py-1 text-sm">
      <span className="font-medium">{detail.name}</span>
      <span className="text-muted-foreground">厂商：{detail.vendor || '—'}</span>
      <span className="inline-flex items-center gap-1 text-muted-foreground">
        上游分组倍率：{formatNumber(detail.discount, 3)}
        <Badge tone={detail.discount_source === 'manual' ? 'warning' : 'neutral'}>
          {detail.discount_source === 'manual' ? '手动' : '自动'}
        </Badge>
      </span>
      {detail.derived_median !== null && (
        <span className="text-muted-foreground">
          按当前上游价推算倍率为 {formatNumber(detail.derived_median, 3)}，自动来源会在倍率同步后采用
        </span>
      )}
      <span className="text-muted-foreground">上游货币汇率（元/$）：{formatNumber(detail.currency_rate)}</span>
      <span className="text-muted-foreground">测试模型：{detail.test_model || '—'}</span>
      <span className="ml-auto"><CacheOverridePopover channelId={detail.channel_id} /></span>
    </div>
  );
}

function useModelStats(channelId: number | null, statsWindow: StatsWindow) {
  const query = useChannelModelStats(channelId, statsWindow);
  return useMemo(
    () => new Map((query.data?.items ?? []).map((item) => [item.model, item])),
    [query.data],
  );
}

export function ModelDetail({ row, statsWindow }: { row: ChannelRow | null; statsWindow: StatsWindow }) {
  const query = useChannelModels(row?.id ?? null);
  const modelStats = useModelStats(row?.id ?? null, statsWindow);
  const columns = useModelDetailColumns(row?.id ?? 0, query.data?.discount ?? row?.discount ?? 0, modelStats);
  if (!row) {
    return (
      <div className="flex h-full items-center justify-center rounded border bg-card text-muted-foreground">
        点击上表中的渠道，查看它的模型明细
      </div>
    );
  }
  if (row.id < 0) {
    return (
      <div className="flex h-full items-center justify-center rounded border bg-card text-muted-foreground">
        新建草稿的渠道应用变更后才有模型明细
      </div>
    );
  }
  const detail = query.data;
  const models = detail?.models ?? NO_MODELS;
  return (
    <div className="flex h-full min-h-0 flex-col rounded border bg-card">
      {detail ? <Header detail={detail} /> : (
        <div className="shrink-0 border-b bg-muted/50 px-3 py-1 text-sm text-muted-foreground">
          {query.isError ? query.error.message : '加载中…'}
        </div>
      )}
      <DataTable<ModelRow>
        layoutId="channels.models"
        className="min-h-0 flex-1 rounded-none border-0"
        data={models}
        columns={columns}
        getRowId={modelRowId}
        status={{ loading: query.isLoading, error: query.isError ? query.error.message : null, empty: '该渠道没有模型' }}
      />
    </div>
  );
}
