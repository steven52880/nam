import { describe, expect, it } from 'vitest';
import type { ChannelStatsItem, ModelCost } from '@/api/types';
import { EMPTY_FILTERS } from './filter';
import { makeChannel } from './testRows';
import { buildChannelView, modelOptionsOf, summarizeGroup } from './viewModel';

function stat(channelId: number, over: Partial<ChannelStatsItem> = {}): ChannelStatsItem {
  return {
    channel_id: channelId,
    requests: 0,
    success: 0,
    fail: 0,
    success_rate: null,
    wilson: null,
    fail_by_class: {},
    frt_p50: null,
    frt_p95: null,
    speed_p50: null,
    speed_p95: null,
    speed_estimated: false,
    cache_hit: null,
    prompt_tokens: 0,
    completion_tokens: 0,
    cache_read_tokens: 0,
    cache_write_tokens: 0,
    cost_cny: 0,
    revenue_cny: 0,
    priced_requests: 0,
    effective_cost_price: null,
    measured_cost_price: null,
    margin: null,
    std_cost_price: null,
    price_tier: null,
    quality_tier: 'insufficient',
    ...over,
  };
}

describe('buildChannelView', () => {
  const rows = [
    makeChannel({ id: 1, name: 'alpha', status: 1, channel_cost: 1, unit_cost: 0.1 }),
    makeChannel({ id: 2, name: 'beta', status: 2, channel_cost: 3, unit_cost: 0.3 }),
    makeChannel({ id: 3, name: 'gamma', status: 1, channel_cost: 5, unit_cost: null }),
  ];

  it('合并统计：有统计的行带 stat，没有的为 null', () => {
    const s = stat(1, { requests: 10 });
    const view = buildChannelView({ rows, stats: new Map([[1, s]]), costs: null, model: null, filters: EMPTY_FILTERS });
    expect(view.rows.map((r) => r.stat)).toEqual([s, null, null]);
    expect(view.rows[0].name).toBe('alpha');
  });

  it('筛选：状态、搜索', () => {
    const byStatus = buildChannelView({ rows, stats: new Map(), costs: null, model: null, filters: { ...EMPTY_FILTERS, status: '1' } });
    expect(byStatus.rows.map((r) => r.id)).toEqual([1, 3]);
    const bySearch = buildChannelView({ rows, stats: new Map(), costs: null, model: null, filters: { ...EMPTY_FILTERS, search: ' BET ' } });
    expect(bySearch.rows.map((r) => r.id)).toEqual([2]);
  });

  it('成本区间只按可见行计算，空值忽略', () => {
    const all = buildChannelView({ rows, stats: new Map(), costs: null, model: null, filters: EMPTY_FILTERS });
    expect(all.ranges.channelCost).toEqual({ min: 1, max: 5 });
    expect(all.ranges.unitCost).toEqual({ min: 0.1, max: 0.3 });
    const some = buildChannelView({ rows, stats: new Map(), costs: null, model: null, filters: { ...EMPTY_FILTERS, status: '1' } });
    expect(some.ranges.channelCost).toEqual({ min: 1, max: 5 });
    expect(some.ranges.unitCost).toBeNull();
  });

  it('实测成本、标准负载成本区间来自 stat', () => {
    const stats = new Map([
      [1, stat(1, { effective_cost_price: 0.5, std_cost_price: 2 })],
      [2, stat(2, { effective_cost_price: 1.5, std_cost_price: 4 })],
    ]);
    const view = buildChannelView({ rows, stats, costs: null, model: null, filters: EMPTY_FILTERS });
    expect(view.ranges.effectiveCost).toEqual({ min: 0.5, max: 1.5 });
    expect(view.ranges.stdCost).toEqual({ min: 2, max: 4 });
  });

  it('草稿新建渠道（负数 id）保留，没有统计', () => {
    const draft = makeChannel({ id: -1, name: 'new', sync_state: 'draft_create', unit_cost: null, channel_cost: null });
    const view = buildChannelView({ rows: [...rows, draft], stats: new Map([[1, stat(1)]]), costs: null, model: null, filters: EMPTY_FILTERS });
    const row = view.rows.find((r) => r.id === -1);
    expect(row?.stat).toBeNull();
    expect(row?.sync_state).toBe('draft_create');
    expect(view.ranges.channelCost).toEqual({ min: 1, max: 5 });
  });

  it('不改动原行', () => {
    const view = buildChannelView({ rows, stats: new Map(), costs: null, model: null, filters: EMPTY_FILTERS });
    expect(view.rows[0]).not.toBe(rows[0]);
    expect('stat' in rows[0]).toBe(false);
  });
});

describe('buildChannelView：没选模型', () => {
  const rows = [makeChannel({ id: 1, discount: 0.8, channel_cost: 5.6, unit_cost: 0.2, unit_cost_model: 'gpt-4o' })];

  it('view 取渠道原值，hasModel 恒为 true；传入的 costs 被忽略', () => {
    const ignored: ModelCost[] = [{ channel_id: 1, has_model: false, discount: null, channel_cost: null, unit_cost: null }];
    const view = buildChannelView({ rows, stats: new Map(), costs: ignored, model: null, filters: EMPTY_FILTERS });
    expect(view.rows[0].view).toEqual({ hasModel: true, discount: 0.8, channelCost: 5.6, unitCost: 0.2, unitCostModel: 'gpt-4o' });
  });
});

describe('buildChannelView：选了模型', () => {
  const rows = [
    makeChannel({ id: 1, models: ['m1', 'm2'], discount: 0.8, channel_cost: 5.6, unit_cost: 0.2 }),
    makeChannel({ id: 2, models: ['m1'], discount: 0.9, channel_cost: 6.3, unit_cost: 0.3 }),
    makeChannel({ id: 3, models: ['m2'], discount: 0.7, channel_cost: 4.9, unit_cost: 0.1 }),
  ];
  const costs: ModelCost[] = [
    { channel_id: 1, has_model: true, discount: 0.5, channel_cost: 3.5, unit_cost: 0.05 },
    { channel_id: 2, has_model: true, discount: 0.6, channel_cost: 4.2, unit_cost: 0.08 },
    { channel_id: 3, has_model: false, discount: null, channel_cost: null, unit_cost: null },
  ];

  it('成本视图取该模型的折扣、渠道成本、单次成本，unitCostModel 是所选模型', () => {
    const view = buildChannelView({ rows, stats: new Map(), costs, model: 'm1', filters: EMPTY_FILTERS });
    expect(view.rows[0].view).toEqual({ hasModel: true, discount: 0.5, channelCost: 3.5, unitCost: 0.05, unitCostModel: 'm1' });
    expect(view.rows[1].view).toEqual({ hasModel: true, discount: 0.6, channelCost: 4.2, unitCost: 0.08, unitCostModel: 'm1' });
    // 渠道自己的字段不被改动
    expect(view.rows[0].channel_cost).toBe(5.6);
  });

  it('渠道不含该模型：hasModel 为 false，成本值为空，且不参与成本区间', () => {
    const view = buildChannelView({ rows, stats: new Map(), costs, model: 'm1', filters: EMPTY_FILTERS });
    expect(view.rows[2].view).toEqual({ hasModel: false, discount: null, channelCost: null, unitCost: null, unitCostModel: 'm1' });
    expect(view.ranges.channelCost).toEqual({ min: 3.5, max: 4.2 });
    expect(view.ranges.unitCost).toEqual({ min: 0.05, max: 0.08 });
  });

  it('不含该模型的渠道，其统计不参与实测成本区间', () => {
    const stats = new Map([
      [1, stat(1, { effective_cost_price: 1, std_cost_price: 2 })],
      [2, stat(2, { effective_cost_price: 1.5, std_cost_price: 4 })],
      [3, stat(3, { effective_cost_price: 9, std_cost_price: 9 })],
    ]);
    const view = buildChannelView({ rows, stats, costs, model: 'm1', filters: EMPTY_FILTERS });
    expect(view.ranges.effectiveCost).toEqual({ min: 1, max: 1.5 });
    expect(view.ranges.stdCost).toEqual({ min: 2, max: 4 });
  });

  it('成本数据还没到（costs 为 null）：成本值为空，不用渠道汇总值冒充；hasModel 按渠道自己的模型列表判断', () => {
    const view = buildChannelView({ rows, stats: new Map(), costs: null, model: 'm1', filters: EMPTY_FILTERS });
    expect(view.rows.map((r) => r.view.hasModel)).toEqual([true, true, false]);
    for (const r of view.rows) {
      expect(r.view).toMatchObject({ discount: null, channelCost: null, unitCost: null, unitCostModel: 'm1' });
    }
    expect(view.ranges.channelCost).toBeNull();
    expect(view.ranges.unitCost).toBeNull();
  });

  it('草稿新建渠道（负数 id）：接口给了就用接口的值', () => {
    const draft = makeChannel({ id: -1, name: 'new', models: ['m1'], sync_state: 'draft_create', channel_cost: null, unit_cost: null });
    const withCost: ModelCost[] = [...costs, { channel_id: -1, has_model: true, discount: 0.4, channel_cost: 2.8, unit_cost: 0.02 }];
    const view = buildChannelView({ rows: [...rows, draft], stats: new Map(), costs: withCost, model: 'm1', filters: EMPTY_FILTERS });
    expect(view.rows.find((r) => r.id === -1)?.view).toEqual({ hasModel: true, discount: 0.4, channelCost: 2.8, unitCost: 0.02, unitCostModel: 'm1' });
  });

  it('草稿新建渠道（负数 id）：接口没给这一行时按渠道自己的模型列表判断，成本值为空', () => {
    const has = makeChannel({ id: -1, models: ['m1'], sync_state: 'draft_create' });
    const not = makeChannel({ id: -2, models: ['other'], sync_state: 'draft_create' });
    const view = buildChannelView({ rows: [has, not], stats: new Map(), costs, model: 'm1', filters: EMPTY_FILTERS });
    expect(view.rows[0].view).toEqual({ hasModel: true, discount: null, channelCost: null, unitCost: null, unitCostModel: 'm1' });
    expect(view.rows[1].view.hasModel).toBe(false);
    expect(view.rows[0].stat).toBeNull();
  });
});

describe('模型已从渠道移除，但统计里还有历史数据', () => {
  // 渠道 1 含 m1；渠道 2 以前跑过 m1、后来移除了，统计接口仍返回它的历史数据
  const rows = [
    makeChannel({ id: 1, models: ['m1'], status: 1 }),
    makeChannel({ id: 2, models: ['other'], status: 1 }),
  ];
  const stats = new Map([
    [1, stat(1, { requests: 10, success: 9, cost_cny: 1, effective_cost_price: 1, std_cost_price: 2 })],
    [2, stat(2, { requests: 100, success: 0, cost_cny: 50, effective_cost_price: 9, std_cost_price: 9 })],
  ]);
  const loaded: ModelCost[] = [
    { channel_id: 1, has_model: true, discount: 0.5, channel_cost: 3.5, unit_cost: 0.05 },
    { channel_id: 2, has_model: false, discount: null, channel_cost: null, unit_cost: null },
  ];

  for (const [name, costs] of [['加载中或请求失败（costs 为 null）', null], ['加载成功', loaded]] as const) {
    it(`${name}：hasModel 为 false，不参与成本区间，组摘要不累加`, () => {
      const view = buildChannelView({ rows, stats, costs, model: 'm1', filters: EMPTY_FILTERS });
      expect(view.rows.map((r) => r.view.hasModel)).toEqual([true, false]);
      expect(view.ranges.effectiveCost).toBeNull(); // 只剩渠道 1 一个值
      expect(summarizeGroup(view.rows, true)).toBe('2 个渠道 · 启用 2 · 平均成功率 90.0% · 成本 ¥1.00');
    });
  }

  it('没选模型时两个渠道的统计都计入组摘要', () => {
    const view = buildChannelView({ rows, stats, costs: null, model: null, filters: EMPTY_FILTERS });
    expect(summarizeGroup(view.rows, true)).toBe('2 个渠道 · 启用 2 · 平均成功率 8.2% · 成本 ¥51.00');
  });
});

describe('modelOptionsOf', () => {
  it('去重并统计每个模型的渠道数（同一渠道里重复的模型只算一次）', () => {
    const rows = [
      makeChannel({ id: 1, models: ['b', 'a', 'a'] }),
      makeChannel({ id: 2, models: ['a'] }),
      makeChannel({ id: -1, models: ['c'] }),
    ];
    expect(modelOptionsOf(rows)).toEqual([
      { model: 'a', channelCount: 2 },
      { model: 'b', channelCount: 1 },
      { model: 'c', channelCount: 1 },
    ]);
  });
});

describe('summarizeGroup', () => {
  it('渠道数、启用数、按请求数加权的成功率、成本合计', () => {
    const rows = buildChannelView({
      rows: [makeChannel({ id: 1, status: 1 }), makeChannel({ id: 2, status: 2 })],
      stats: new Map([
        [1, stat(1, { requests: 10, success: 9, cost_cny: 1.5 })],
        [2, stat(2, { requests: 30, success: 15, cost_cny: 2 })],
      ]),
      costs: null,
      model: null,
      filters: EMPTY_FILTERS,
    }).rows;
    expect(summarizeGroup(rows, true)).toBe('2 个渠道 · 启用 1 · 平均成功率 60.0% · 成本 ¥3.50');
  });

  it('没有统计数据时成功率和成本显示 —', () => {
    const rows = buildChannelView({ rows: [makeChannel({ id: 1 })], stats: new Map(), costs: null, model: null, filters: EMPTY_FILTERS }).rows;
    expect(summarizeGroup(rows, false)).toBe('1 个渠道 · 启用 1 · 平均成功率 — · 成本 —');
  });
});
