import { useChannelModels, useChannelModelStats } from '@/api/queries';
import { useMemo } from 'react';
import type { ChannelModels, ChannelRow, ModelRow, StatsWindow } from '@/api/types';
import { DataTable } from '@/components/table';
import { Badge, Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatDiscount, formatGroupRatio, formatNumber } from '@/lib/format';
import { DISCOUNT_SOURCE } from '@/lib/labels';
import { deviationBase } from '@/lib/modelRatio';
import { CacheOverridePopover } from './ModelDiscountControls';
import { modelRowClass, modelRowId, useModelDetailColumns } from './modelDetailColumns';

const NO_MODELS: ModelRow[] = [];

/** 渠道倍率 + 来源；算不出时写「无法推算」，悬停说明原因。 */
function ChannelRatio({ detail }: { detail: ChannelModels }) {
  const def = DISCOUNT_SOURCE[detail.discount_source];
  const text =
    detail.discount === null ? (
      <span>无法推算</span>
    ) : (
      <>
        <span className="tabular-nums text-foreground">{formatDiscount(detail.discount)}</span>
        <Badge tone={def.tone} title={def.hint}>{def.label}</Badge>
      </>
    );
  const node = <span className="inline-flex items-center gap-1 text-muted-foreground">渠道倍率：{text}</span>;
  if (detail.discount !== null || !detail.discount_reason) return node;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{node}</TooltipTrigger>
      <TooltipContent>
        <div>{detail.discount_reason}</div>
        <div>可以在渠道表里手动填写上游分组倍率</div>
      </TooltipContent>
    </Tooltip>
  );
}

function Header({ detail }: { detail: ChannelModels }) {
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-0.5 border-b bg-muted/50 px-3 py-1 text-sm">
      <span className="font-medium">{detail.name}</span>
      <span className="text-muted-foreground">厂商：{detail.vendor || '—'}</span>
      <ChannelRatio detail={detail} />
      <span className="text-muted-foreground">
        上游分组倍率：<span className="tabular-nums">{formatGroupRatio(detail.upstream_group_ratio)}</span>
      </span>
      {detail.derived_median !== null && (
        <span className="text-muted-foreground">
          按上游逐模型价推算：<span className="tabular-nums">{formatNumber(detail.derived_median, 3, { trim: true })}</span>
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
  const detail = query.data;
  const fallback = detail ? detail.discount : (row?.discount ?? null);
  const base = useMemo(() => (detail ? deviationBase(detail) : null), [detail]);
  const columns = useModelDetailColumns(row?.id ?? 0, fallback, base, modelStats);
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
        rowClassName={modelRowClass}
        status={{ loading: query.isLoading, error: query.isError ? query.error.message : null, empty: '该渠道没有模型' }}
      />
    </div>
  );
}
