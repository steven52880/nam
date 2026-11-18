import { describe, expect, it } from 'vitest';
import type { PriceCounts, PriceRow, PriceStatus } from '@/api/types';
import { matchesTab, STATUS_TABS, tabCounts } from './constants';
import { PRICE_STATUS, visiblePriceStatus } from '@/lib/labels';

const row = (status: PriceStatus): PriceRow => ({
  model: 'm',
  vendor: '',
  aliases: [],
  official: null,
  source: '',
  newapi: null,
  deviation: null,
  channel_count: 0,
  status,
});

describe('偏差状态当作正常', () => {
  it('visiblePriceStatus', () => {
    expect(visiblePriceStatus('deviation')).toBe('normal');
    expect(visiblePriceStatus('normal')).toBe('normal');
    expect(visiblePriceStatus('no_price')).toBe('no_price');
    expect(visiblePriceStatus('per_request')).toBe('per_request');
  });
  it('状态页签和字典里都没有「偏差」', () => {
    expect(STATUS_TABS.map((t) => t.label)).not.toContain('偏差');
    expect(Object.values(PRICE_STATUS).map((d) => d.label)).not.toContain('偏差');
  });
  it('「正常」页签包含后端标成偏差的行', () => {
    expect(matchesTab(row('deviation'), 'normal')).toBe(true);
    expect(matchesTab(row('normal'), 'normal')).toBe(true);
    expect(matchesTab(row('no_price'), 'normal')).toBe(false);
    expect(matchesTab(row('deviation'), 'all')).toBe(true);
    expect(matchesTab(row('per_request'), 'per_request')).toBe(true);
  });
  it('数量：偏差并入正常', () => {
    const c: PriceCounts = { all: 10, normal: 5, deviation: 2, no_price: 2, per_request: 1 };
    expect(tabCounts(c)).toEqual({ all: 10, normal: 7, no_price: 2, per_request: 1 });
  });
});
