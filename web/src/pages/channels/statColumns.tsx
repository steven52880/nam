import type { ComponentProps } from 'react';
import type { PriceTier, QualityTier } from '@/api/types';
import { CostCell } from '@/components/cells/CostCell';
import { type CostRange } from '@/components/cells/costColor';
import { Dash } from '@/components/cells/Dash';
import { PercentCell } from '@/components/cells/PercentCell';
import { numberCol, type TableColumn } from '@/components/table';
import { Badge } from '@/components/ui/misc';
import { formatSignificant } from '@/lib/format';
import { FirstTokenCell, SpeedStatCell, SuccessRateCell } from './StatCells';
import type { ChannelViewRow } from './viewModel';

type Row = ChannelViewRow;
type Col = TableColumn<Row>;

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
  /** 当前可见行的实测成本范围，用于颜色梯度。 */
  effectiveCostRange: CostRange | null;
  /** 当前可见行的标准负载成本价范围。 */
  stdCostRange: CostRange | null;
}

/** 统计列读 `row.stat`（渠道汇总统计）；宽度单位 rem。 */
function statColumn(
  id: string,
  header: string,
  accessor: Col['accessor'],
  cell: Col['cell'],
  extra: Partial<Col> = {},
): Col {
  return numberCol<Row>(id, header, accessor, { cell, readOnlyReason: STATS_REASON, width: 5.75, ...extra });
}

/** main：默认显示的统计列；extra：默认隐藏的统计列。 */
export function buildStatColumns({ effectiveCostRange, stdCostRange }: StatColumnOptions): {
  main: Col[];
  extra: Col[];
} {
  return {
    main: [
      statColumn('success_rate', '成功率', (r) => r.stat?.success_rate ?? null, (r) => <SuccessRateCell metrics={r.stat ?? undefined} />),
      statColumn('first_token', '首字延迟', (r) => r.stat?.frt_p50 ?? null, (r) => <FirstTokenCell metrics={r.stat ?? undefined} />, { width: 6.25 }),
      statColumn('speed', '吐字速度', (r) => r.stat?.speed_p50 ?? null, (r) => <SpeedStatCell metrics={r.stat ?? undefined} />, { width: 7 }),
      statColumn('cache_hit', '缓存命中率', (r) => r.stat?.cache_hit ?? null, (r) => <PercentCell value={r.stat?.cache_hit} />, { width: 6.25 }),
      statColumn(
        'effective_cost',
        '实测成本',
        (r) => r.stat?.effective_cost_price ?? null,
        (r) => <CostCell value={r.stat?.effective_cost_price} range={effectiveCostRange} hint={EFFECTIVE_COST_HINT} />,
        { headerTitle: EFFECTIVE_COST_HINT },
      ),
    ],
    extra: [
      statColumn('measured_cost', '定价口径成本价', (r) => r.stat?.measured_cost_price ?? null, (r) => (
        <CostCell value={r.stat?.measured_cost_price} range={null} />
      ), { width: 7.5, hidden: true }),
      statColumn('std_cost', '标准负载成本价', (r) => r.stat?.std_cost_price ?? null, (r) => (
        <CostCell value={r.stat?.std_cost_price} range={stdCostRange} />
      ), { width: 7.5, hidden: true }),
      statColumn('price_tier', '价格档', (r) => {
        const tier = r.stat?.price_tier;
        return tier ? PRICE_TIERS[tier].rank : null;
      }, (r) => {
        const tier = r.stat?.price_tier;
        return tier ? <Badge tone={PRICE_TIERS[tier].tone}>{PRICE_TIERS[tier].label}</Badge> : <Dash />;
      }, { width: 5, hidden: true }),
      statColumn('quality_tier', '质量档', (r) => {
        const item = r.stat;
        return item ? QUALITY_TIERS[item.quality_tier].rank : null;
      }, (r) => {
        const item = r.stat;
        if (!item) return <Dash />;
        const tier = QUALITY_TIERS[item.quality_tier];
        return <Badge tone={tier.tone}>{tier.label}</Badge>;
      }, { width: 5.75, hidden: true }),
      statColumn('cost', '成本（元）', (r) => r.stat?.cost_cny ?? null, (r) => {
        const item = r.stat;
        return item ? <span className="tabular-nums">{formatSignificant(item.cost_cny, 4)}</span> : <Dash />;
      }, { width: 6.25, hidden: true }),
    ],
  };
}
