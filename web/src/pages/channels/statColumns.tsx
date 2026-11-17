import type { ComponentProps } from 'react';
import type { ChannelRow, ChannelStatsItem, PriceTier, QualityTier } from '@/api/types';
import { CostCell } from '@/components/cells/CostCell';
import { type CostRange } from '@/components/cells/costColor';
import { Dash } from '@/components/cells/Dash';
import { PercentCell } from '@/components/cells/PercentCell';
import type { GridColumn } from '@/components/data-grid';
import { Badge } from '@/components/ui/misc';
import { formatSignificant } from '@/lib/format';
import { FirstTokenCell, SpeedStatCell, SuccessRateCell } from './StatCells';

const STATS_REASON = '统计列由系统计算，不能编辑';
const EFFECTIVE_COST_HINT = '每消耗原厂 1 美元（不算缓存）实际花多少元，缓存命中越高越低';

type BadgeTone = NonNullable<ComponentProps<typeof Badge>['tone']>;

const PRICE_TIERS: Record<PriceTier, { label: string; tone: BadgeTone; rank: number }> = {
  cheap: { label: '便宜', tone: 'success', rank: 0 },
  mid: { label: '中', tone: 'neutral', rank: 1 },
  expensive: { label: '贵', tone: 'danger', rank: 2 },
};

const QUALITY_TIERS: Record<QualityTier, { label: string; tone: BadgeTone; rank: number }> = {
  good: { label: '好', tone: 'success', rank: 3 },
  mid: { label: '中', tone: 'warning', rank: 2 },
  bad: { label: '差', tone: 'danger', rank: 1 },
  insufficient: { label: '样本不足', tone: 'neutral', rank: 0 },
};

export interface StatColumnOptions {
  /** 渠道 id → 当前统计时长下的统计。 */
  stats: Map<number, ChannelStatsItem>;
  /** 当前可见行的实测成本范围，用于颜色梯度。 */
  effectiveCostRange: CostRange | null;
  /** 当前可见行的标准负载成本价范围。 */
  stdCostRange: CostRange | null;
}

function statColumn(
  id: string,
  header: string,
  accessor: GridColumn<ChannelRow>['accessor'],
  cell: GridColumn<ChannelRow>['cell'],
  extra: Partial<GridColumn<ChannelRow>> = {},
): GridColumn<ChannelRow> {
  return { id, header, accessor, cell, readOnlyReason: STATS_REASON, width: 90, align: 'right', sortable: true, ...extra };
}

/** main：默认显示的统计列；extra：默认隐藏的统计列。 */
export function buildStatColumns({ stats, effectiveCostRange, stdCostRange }: StatColumnOptions): {
  main: GridColumn<ChannelRow>[];
  extra: GridColumn<ChannelRow>[];
} {
  const of = (row: ChannelRow) => stats.get(row.id);
  return {
    main: [
      statColumn('success_rate', '成功率', (r) => of(r)?.success_rate ?? null, (r) => <SuccessRateCell metrics={of(r)} />),
      statColumn('first_token', '首字延迟', (r) => of(r)?.frt_p50 ?? null, (r) => <FirstTokenCell metrics={of(r)} />, { width: 100 }),
      statColumn('speed', '吐字速度', (r) => of(r)?.speed_p50 ?? null, (r) => <SpeedStatCell metrics={of(r)} />, { width: 110 }),
      statColumn('cache_hit', '缓存命中率', (r) => of(r)?.cache_hit ?? null, (r) => <PercentCell value={of(r)?.cache_hit} />, { width: 100 }),
      statColumn(
        'effective_cost',
        '实测成本',
        (r) => of(r)?.effective_cost_price ?? null,
        (r) => <CostCell value={of(r)?.effective_cost_price} range={effectiveCostRange} hint={EFFECTIVE_COST_HINT} />,
        { headerTitle: EFFECTIVE_COST_HINT },
      ),
    ],
    extra: [
      statColumn('measured_cost', '定价口径成本价', (r) => of(r)?.measured_cost_price ?? null, (r) => (
        <CostCell value={of(r)?.measured_cost_price} range={null} />
      ), { width: 120, hidden: true }),
      statColumn('std_cost', '标准负载成本价', (r) => of(r)?.std_cost_price ?? null, (r) => (
        <CostCell value={of(r)?.std_cost_price} range={stdCostRange} />
      ), { width: 120, hidden: true }),
      statColumn('price_tier', '价格档', (r) => {
        const tier = of(r)?.price_tier;
        return tier ? PRICE_TIERS[tier].rank : null;
      }, (r) => {
        const tier = of(r)?.price_tier;
        return tier ? <Badge tone={PRICE_TIERS[tier].tone}>{PRICE_TIERS[tier].label}</Badge> : <Dash />;
      }, { width: 80, hidden: true }),
      statColumn('quality_tier', '质量档', (r) => {
        const item = of(r);
        return item ? QUALITY_TIERS[item.quality_tier].rank : null;
      }, (r) => {
        const item = of(r);
        if (!item) return <Dash />;
        const tier = QUALITY_TIERS[item.quality_tier];
        return <Badge tone={tier.tone}>{tier.label}</Badge>;
      }, { width: 90, hidden: true }),
      statColumn('cost', '成本（元）', (r) => of(r)?.cost_cny ?? null, (r) => {
        const item = of(r);
        return item ? <span className="tabular-nums">{formatSignificant(item.cost_cny, 4)}</span> : <Dash />;
      }, { width: 100, hidden: true }),
    ],
  };
}
