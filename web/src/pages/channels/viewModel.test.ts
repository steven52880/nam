import { describe, expect, it } from 'vitest';
import type { ChannelStatsItem } from '@/api/types';
import { EMPTY_FILTERS } from './filter';
import { makeChannel } from './testRows';
import { buildChannelView, summarizeGroup } from './viewModel';

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
    const view = buildChannelView({ rows, stats: new Map([[1, s]]), filters: EMPTY_FILTERS });
    expect(view.rows.map((r) => r.stat)).toEqual([s, null, null]);
    expect(view.rows[0].name).toBe('alpha');
  });

  it('筛选：状态、搜索', () => {
    const byStatus = buildChannelView({ rows, stats: new Map(), filters: { ...EMPTY_FILTERS, status: '1' } });
    expect(byStatus.rows.map((r) => r.id)).toEqual([1, 3]);
    const bySearch = buildChannelView({ rows, stats: new Map(), filters: { ...EMPTY_FILTERS, search: ' BET ' } });
    expect(bySearch.rows.map((r) => r.id)).toEqual([2]);
  });

  it('成本区间只按可见行计算，空值忽略', () => {
    const all = buildChannelView({ rows, stats: new Map(), filters: EMPTY_FILTERS });
    expect(all.ranges.channelCost).toEqual({ min: 1, max: 5 });
    expect(all.ranges.unitCost).toEqual({ min: 0.1, max: 0.3 });
    const some = buildChannelView({ rows, stats: new Map(), filters: { ...EMPTY_FILTERS, status: '1' } });
    expect(some.ranges.channelCost).toEqual({ min: 1, max: 5 });
    expect(some.ranges.unitCost).toBeNull();
  });

  it('实测成本、标准负载成本区间来自 stat', () => {
    const stats = new Map([
      [1, stat(1, { effective_cost_price: 0.5, std_cost_price: 2 })],
      [2, stat(2, { effective_cost_price: 1.5, std_cost_price: 4 })],
    ]);
    const view = buildChannelView({ rows, stats, filters: EMPTY_FILTERS });
    expect(view.ranges.effectiveCost).toEqual({ min: 0.5, max: 1.5 });
    expect(view.ranges.stdCost).toEqual({ min: 2, max: 4 });
  });

  it('草稿新建渠道（负数 id）保留，没有统计', () => {
    const draft = makeChannel({ id: -1, name: 'new', sync_state: 'draft_create', unit_cost: null, channel_cost: null });
    const view = buildChannelView({ rows: [...rows, draft], stats: new Map([[1, stat(1)]]), filters: EMPTY_FILTERS });
    const row = view.rows.find((r) => r.id === -1);
    expect(row?.stat).toBeNull();
    expect(row?.sync_state).toBe('draft_create');
    expect(view.ranges.channelCost).toEqual({ min: 1, max: 5 });
  });

  it('不改动原行', () => {
    const view = buildChannelView({ rows, stats: new Map(), filters: EMPTY_FILTERS });
    expect(view.rows[0]).not.toBe(rows[0]);
    expect('stat' in rows[0]).toBe(false);
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
      filters: EMPTY_FILTERS,
    }).rows;
    expect(summarizeGroup(rows, true)).toBe('2 个渠道 · 启用 1 · 平均成功率 60.0% · 成本 ¥3.50');
  });

  it('没有统计数据时成功率和成本显示 —', () => {
    const rows = buildChannelView({ rows: [makeChannel({ id: 1 })], stats: new Map(), filters: EMPTY_FILTERS }).rows;
    expect(summarizeGroup(rows, false)).toBe('1 个渠道 · 启用 1 · 平均成功率 — · 成本 —');
  });
});
