import type { ChannelRow, ChannelStatsItem, ModelCost } from '@/api/types';
import { costRangeOf, type CostRange } from '@/components/cells/costColor';
import { formatPercent } from '@/lib/format';
import { filterChannels, type ChannelFilters } from './filter';

/** 成本相关的视图值：没选模型时取渠道原值，选了模型时取该模型的值。 */
export interface ChannelCostView {
  /** 渠道是否包含所选模型；没选模型时恒为 true。 */
  hasModel: boolean;
  /** 选了模型时是模型级生效折扣；成本数据还没到时为 null。 */
  discount: number | null;
  channelCost: number | null;
  unitCost: number | null;
  unitCostModel: string;
}

/** 表格行：渠道 + 统计（选了模型：该模型的统计；否则渠道汇总；没有统计为 null）+ 成本视图。 */
export type ChannelViewRow = ChannelRow & { stat: ChannelStatsItem | null; view: ChannelCostView };

export interface ChannelRanges {
  channelCost: CostRange | null;
  unitCost: CostRange | null;
  effectiveCost: CostRange | null;
  stdCost: CostRange | null;
}

function costViewOf(r: ChannelRow, model: string | null, costs: Map<number, ModelCost> | null): ChannelCostView {
  if (model === null) {
    return { hasModel: true, discount: r.discount, channelCost: r.channel_cost, unitCost: r.unit_cost, unitCostModel: r.unit_cost_model };
  }
  // 选了模型但成本数据还没到（加载中或请求失败）：不用渠道汇总值冒充，成本列显示 —；
  // 是否包含该模型按渠道自己的模型列表判断，已移除该模型的渠道不显示历史统计
  if (costs === null) {
    return { hasModel: r.models.includes(model), discount: null, channelCost: null, unitCost: null, unitCostModel: model };
  }
  const c = costs.get(r.id);
  // 接口没给这一行（例如刚新建的草稿渠道）：按渠道自己的模型列表判断，成本值留空
  const hasModel = c ? c.has_model : r.models.includes(model);
  if (!c || !c.has_model) {
    return { hasModel, discount: null, channelCost: null, unitCost: null, unitCostModel: model };
  }
  return { hasModel, discount: c.discount, channelCost: c.channel_cost, unitCost: c.unit_cost, unitCostModel: model };
}

/**
 * 筛选 → 合并统计和成本视图 → 按可见行计算成本颜色范围。
 * model 为 null：统计是渠道汇总，成本取渠道原值（costs 不用）；
 * model 不为 null：costs 为 null 表示还在加载。
 */
export function buildChannelView(input: {
  rows: ChannelRow[];
  stats: ReadonlyMap<number, ChannelStatsItem>;
  costs: ModelCost[] | null;
  model: string | null;
  filters: ChannelFilters;
}): { rows: ChannelViewRow[]; ranges: ChannelRanges } {
  const model = input.model;
  const costMap = input.costs ? new Map(input.costs.map((c) => [c.channel_id, c])) : null;
  const rows = filterChannels(input.rows, input.filters).map<ChannelViewRow>((r) => ({
    ...r,
    stat: input.stats.get(r.id) ?? null,
    view: costViewOf(r, model, costMap),
  }));
  return {
    rows,
    ranges: {
      channelCost: costRangeOf(rows.map((r) => r.view.channelCost)),
      unitCost: costRangeOf(rows.map((r) => r.view.unitCost)),
      effectiveCost: costRangeOf(rows.map((r) => (r.view.hasModel ? r.stat?.effective_cost_price ?? null : null))),
      stdCost: costRangeOf(rows.map((r) => (r.view.hasModel ? r.stat?.std_cost_price ?? null : null))),
    },
  };
}

/** 模型选项：全部渠道里的模型去重，并统计每个模型被多少个渠道包含；按模型名排序。 */
export function modelOptionsOf(rows: ChannelRow[]): Array<{ model: string; channelCount: number }> {
  const counts = new Map<string, number>();
  for (const r of rows) {
    for (const m of new Set(r.models)) counts.set(m, (counts.get(m) ?? 0) + 1);
  }
  return [...counts]
    .map(([model, channelCount]) => ({ model, channelCount }))
    .sort((a, b) => (a.model < b.model ? -1 : a.model > b.model ? 1 : 0));
}

/** 组标题行摘要：渠道数、启用数、平均成功率（按请求数加权）、成本合计。hasStats = 统计接口有数据。 */
export function summarizeGroup(rows: ChannelViewRow[], hasStats: boolean): string {
  let requests = 0;
  let success = 0;
  let cost = 0;
  for (const row of rows) {
    const item = row.view.hasModel ? row.stat : null;
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
