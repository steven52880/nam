import { useMemo } from 'react';
import type { ModelRow, ModelStatsItem } from '@/api/types';
import { Dash } from '@/components/cells/Dash';
import { OfficialPriceText } from '@/components/PriceText';
import { customCol, numberCol, textCol, type TableColumn } from '@/components/table';
import { Badge, Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatNumber, formatPercent, formatSignificant } from '@/lib/format';
import { EFFECTIVE_SOURCE } from '@/lib/labels';
import { deviationHint, type DeviationBase } from '@/lib/modelRatio';
import { cn } from '@/lib/utils';
import { DiscountCell } from './ModelDiscountControls';
import { FirstTokenCell, SpeedStatCell, SuccessRateCell } from './StatCells';

function DerivedCell({ row }: { row: ModelRow }) {
  if (row.derived === null) {
    return <span className="text-xs text-muted-foreground">{row.reason || '无法推算'}</span>;
  }
  return <span className="tabular-nums">{formatNumber(row.derived, 3)}</span>;
}

/** 生效倍率：数值 + 来源；上游标价偏离基准时用醒目颜色并在悬停里说明。 */
function EffectiveCell({ row, base }: { row: ModelRow; base: DeviationBase | null }) {
  if (row.effective === null) {
    return (
      <span className="text-muted-foreground" title={row.reason || undefined}>
        {EFFECTIVE_SOURCE.none.label}
      </span>
    );
  }
  const source = EFFECTIVE_SOURCE[row.effective_source];
  const content = (
    <span className="inline-flex items-center justify-end gap-1">
      {row.deviates && (
        <Badge tone="warning">偏离 {formatPercent(row.deviation, { signed: true })}</Badge>
      )}
      <Badge tone={source.tone} title={source.hint}>{source.label}</Badge>
      <span className={cn('tabular-nums', row.deviates && 'font-semibold text-warning')}>
        {formatNumber(row.effective, 3, { trim: true })}
      </span>
    </span>
  );
  if (!row.deviates) return content;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{content}</TooltipTrigger>
      <TooltipContent>{deviationHint(row, base)}</TooltipContent>
    </Tooltip>
  );
}

function CostCell({ row }: { row: ModelRow }) {
  if (row.cost_per_request !== null) return <span className="tabular-nums">{formatSignificant(row.cost_per_request)} 元/次</span>;
  if (row.cost_in_per_m === null && row.cost_out_per_m === null) return <Dash />;
  return (
    <span className="tabular-nums">
      {formatSignificant(row.cost_in_per_m)} / {formatSignificant(row.cost_out_per_m)}
    </span>
  );
}

export function modelRowId(row: ModelRow): string {
  return row.model;
}

/** 偏离的行：整行淡色底，和渠道表里的草稿（黄角标）、错误（红字）区分开。 */
export function modelRowClass(row: ModelRow): string | undefined {
  return row.deviates ? 'bg-warning/20' : undefined;
}

/**
 * 渠道模型明细的列。channelId、fallback（渠道倍率）、base（偏离基准）、modelStats 变化时才重建。
 */
export function useModelDetailColumns(
  channelId: number,
  fallback: number | null,
  base: DeviationBase | null,
  modelStats: Map<string, ModelStatsItem>,
): TableColumn<ModelRow>[] {
  return useMemo<TableColumn<ModelRow>[]>(() => [
    textCol('model', '模型名', (m) => m.model, { width: 14 }),
    customCol<ModelRow>('official_price', '原厂价（$/M）', {
      width: 19.5,
      align: 'left',
      sortType: 'number',
      accessor: (m) => m.price?.input ?? null,
      cell: (m) => <OfficialPriceText price={m.price} />,
    }),
    numberCol('upstream_price', '上游价', (m) => m.upstream_input, {
      width: 6,
      cell: (m) => formatSignificant(m.upstream_input),
    }),
    customCol<ModelRow>('derived_discount', '推算倍率', {
      width: 7,
      align: 'right',
      sortType: 'number',
      accessor: (m) => m.derived,
      cell: (m) => <DerivedCell row={m} />,
    }),
    customCol<ModelRow>('discount_override', '上游分组倍率覆盖', {
      width: 10,
      align: 'right',
      sortType: 'number',
      accessor: (m) => m.override,
      cell: (m) => <DiscountCell channelId={channelId} model={m.model} override={m.override} fallback={fallback} />,
    }),
    customCol<ModelRow>('effective', '生效倍率', {
      width: 15,
      align: 'right',
      sortType: 'number',
      headerTitle: '这个模型实际用来算成本的倍率，后面的小标记是来源。上游给这个模型的标价和原厂价不一致时整行标色。',
      accessor: (m) => m.effective,
      cell: (m) => <EffectiveCell row={m} base={base} />,
    }),
    customCol<ModelRow>('cost', '成本（元）', {
      width: 9,
      align: 'right',
      sortType: 'number',
      accessor: (m) => m.cost_per_request ?? m.cost_in_per_m,
      cell: (m) => <CostCell row={m} />,
    }),
    numberCol('requests', '请求数', (m) => modelStats.get(m.model)?.requests ?? null, {
      width: 5,
      cell: (m) => {
        const metrics = modelStats.get(m.model);
        return metrics ? metrics.requests : <Dash />;
      },
    }),
    customCol<ModelRow>('success_rate', '成功率', {
      width: 5.5,
      align: 'right',
      sortType: 'number',
      accessor: (m) => modelStats.get(m.model)?.success_rate ?? null,
      cell: (m) => <SuccessRateCell metrics={modelStats.get(m.model)} />,
    }),
    customCol<ModelRow>('first_token', '首字延迟', {
      width: 6,
      align: 'right',
      sortType: 'number',
      accessor: (m) => modelStats.get(m.model)?.frt_p50 ?? null,
      cell: (m) => <FirstTokenCell metrics={modelStats.get(m.model)} />,
    }),
    customCol<ModelRow>('speed', '吐字速度', {
      width: 6.5,
      align: 'right',
      sortType: 'number',
      accessor: (m) => modelStats.get(m.model)?.speed_p50 ?? null,
      cell: (m) => <SpeedStatCell metrics={modelStats.get(m.model)} />,
    }),
  ], [channelId, fallback, base, modelStats]);
}
