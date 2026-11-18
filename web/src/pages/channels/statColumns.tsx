import type { ComponentProps } from 'react';
import type { ChannelStatsItem, PriceTier, QualityTier } from '@/api/types';
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
const EFFECTIVE_COST_HINT =
  '= 渠道真实请求的实际花费 ÷ 同样请求不算缓存、按原厂价计的费用（统一成人民币后相除），缓存命中越高越低。例：实际花 0.4 元，不算缓存的原厂价 20 元 → 2%';
const MEASURED_COST_HINT =
  '= 实际花费 ÷ 算上缓存优惠的原厂价。正常应约等于上游渠道售价，明显偏高说明上游多扣了。例：实际花 0.5 元，带缓存优惠的原厂价 20 元 → 2.5%';
const STD_COST_HINT =
  '= 用同厂商所有渠道近 7 天的平均 token 比例（输入 / 缓存 / 输出），按本渠道的倍率、汇率、缓存价理论计算的 实际花费 ÷ 不算缓存的原厂价。各渠道用同一份负载公平比价，没流量也能算，价格档按它划分。例：2.1%';
const NEEDS_MODEL_HINT = '选择模型后显示';

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
  /** 当前选中的模型；null = 渠道汇总（首字延迟、吐字速度这两列只在选了模型后显示）。 */
  model: string | null;
  /** 当前可见行的实际负载成本范围，用于颜色梯度。 */
  effectiveCostRange: CostRange | null;
  /** 当前可见行的标准负载成本范围。 */
  stdCostRange: CostRange | null;
}

/** 统计列读 `row.stat`；渠道不含所选模型（hasModel = false）时整行统计显示「—」。 */
function statOf(r: Row): ChannelStatsItem | null {
  return r.view.hasModel ? r.stat : null;
}

/** 统计列读 `row.stat`；宽度单位 rem。 */
function statColumn(
  id: string,
  header: string,
  accessor: Col['accessor'],
  cell: Col['cell'],
  extra: Partial<Col> = {},
): Col {
  return numberCol<Row>(id, header, accessor, { cell, readOnlyReason: STATS_REASON, width: 5.75, ...extra });
}

/** 只有选了模型才有意义的统计列：没选模型时整列显示「—」，表头提示"选择模型后显示"。 */
function modelOnlyColumn(
  model: string | null,
  id: string,
  header: string,
  pick: (s: ChannelStatsItem) => number | null,
  cell: Col['cell'],
  width: number,
): Col {
  if (model === null) {
    return statColumn(id, header, () => null, () => <Dash />, { width, headerTitle: NEEDS_MODEL_HINT });
  }
  return statColumn(id, header, (r) => {
    const s = statOf(r);
    return s ? pick(s) : null;
  }, cell, { width });
}

/** main：默认显示的统计列；extra：默认隐藏的统计列。 */
export function buildStatColumns({ model, effectiveCostRange, stdCostRange }: StatColumnOptions): {
  main: Col[];
  extra: Col[];
} {
  return {
    main: [
      statColumn('success_rate', '成功率', (r) => statOf(r)?.success_rate ?? null, (r) => <SuccessRateCell metrics={statOf(r) ?? undefined} />),
      modelOnlyColumn(model, 'first_token', '首字延迟', (s) => s.frt_p50, (r) => <FirstTokenCell metrics={statOf(r) ?? undefined} />, 6.25),
      modelOnlyColumn(model, 'speed', '吐字速度', (s) => s.speed_p50, (r) => <SpeedStatCell metrics={statOf(r) ?? undefined} />, 7),
      statColumn('cache_hit', '缓存命中率', (r) => statOf(r)?.cache_hit ?? null, (r) => <PercentCell value={statOf(r)?.cache_hit} />, { width: 6.25 }),
      statColumn(
        'effective_cost',
        '实际负载成本',
        (r) => statOf(r)?.effective_cost_price ?? null,
        (r) => <CostCell percent value={statOf(r)?.effective_cost_price} range={effectiveCostRange} />,
        { headerTitle: EFFECTIVE_COST_HINT, width: 7 },
      ),
    ],
    extra: [
      statColumn('measured_cost', '实际计费倍率', (r) => statOf(r)?.measured_cost_price ?? null, (r) => (
        <CostCell percent value={statOf(r)?.measured_cost_price} range={null} />
      ), { width: 7.5, hidden: true, headerTitle: MEASURED_COST_HINT }),
      statColumn('std_cost', '标准负载成本', (r) => statOf(r)?.std_cost_price ?? null, (r) => (
        <CostCell percent value={statOf(r)?.std_cost_price} range={stdCostRange} />
      ), { width: 7.5, hidden: true, headerTitle: STD_COST_HINT }),
      statColumn('price_tier', '价格档', (r) => {
        const tier = statOf(r)?.price_tier;
        return tier ? PRICE_TIERS[tier].rank : null;
      }, (r) => {
        const tier = statOf(r)?.price_tier;
        return tier ? <Badge tone={PRICE_TIERS[tier].tone}>{PRICE_TIERS[tier].label}</Badge> : <Dash />;
      }, { width: 5, hidden: true }),
      statColumn('quality_tier', '质量档', (r) => {
        const item = statOf(r);
        return item ? QUALITY_TIERS[item.quality_tier].rank : null;
      }, (r) => {
        const item = statOf(r);
        if (!item) return <Dash />;
        const tier = QUALITY_TIERS[item.quality_tier];
        return <Badge tone={tier.tone}>{tier.label}</Badge>;
      }, { width: 5.75, hidden: true }),
      statColumn('cost', '成本（元）', (r) => statOf(r)?.cost_cny ?? null, (r) => {
        const item = statOf(r);
        return item ? <span className="tabular-nums">{formatSignificant(item.cost_cny, 4)}</span> : <Dash />;
      }, { width: 6.25, hidden: true }),
    ],
  };
}
