import { describe, expect, it } from 'vitest';
import {
  isEmptyPrice,
  isPerRequestPrice,
  priceSlots,
  priceTiers,
  priceTitle,
  SLOT_GAP_CH,
  SLOT_SPACE_CH,
  slotsWidthCh,
} from './priceSlots';

const tokens = { input: 3, output: 15, cache_read: null, per_request: null };

describe('priceSlots', () => {
  it('官方价：入 / 出 / 读 / 写 / 写1h 五个槽位，缺值保持 null 占位', () => {
    const slots = priceSlots({ ...tokens, cache_write_5m: 3.75, cache_write_1h: null });
    expect(slots.map((s) => s.label)).toEqual(['入', '出', '读', '写', '写1h']);
    expect(slots.map((s) => s.value)).toEqual([3, 15, null, 3.75, null]);
  });

  it('NewAPI 当前价没有写1h 字段：只有四个槽位', () => {
    const slots = priceSlots({ ...tokens, cache_write_5m: null });
    expect(slots.map((s) => s.key)).toEqual(['input', 'output', 'cache_read', 'cache_write_5m']);
  });

  it('没有写入字段时只有入 / 出 / 读', () => {
    expect(priceSlots(tokens).map((s) => s.key)).toEqual(['input', 'output', 'cache_read']);
  });

  it('按次价：只有「按次」一个槽位', () => {
    const p = { input: null, output: null, cache_read: null, per_request: 0.04 };
    expect(isPerRequestPrice(p)).toBe(true);
    expect(priceSlots(p).map((s) => [s.label, s.value])).toEqual([['按次', 0.04]]);
  });

  it('有 token 价时即使带按次价也按 token 槽位显示', () => {
    expect(isPerRequestPrice({ ...tokens, per_request: 1 })).toBe(false);
  });

  it('全空判断', () => {
    expect(isEmptyPrice({ input: null, output: null, cache_read: null, per_request: null })).toBe(true);
    expect(isEmptyPrice(tokens)).toBe(false);
    expect(isEmptyPrice({ input: null, output: null, cache_read: null, per_request: 1 })).toBe(false);
  });
});

describe('slotsWidthCh / priceTiers', () => {
  it('槽位宽度 = 标签 + 数值，槽位之间加间距', () => {
    const slots = priceSlots(tokens);
    expect(slots[0].ch).toBe(slots[0].valueCh + SLOT_SPACE_CH + slots[0].labelCh);
    expect(slotsWidthCh(slots)).toBe(slots.reduce((n, s) => n + s.ch, 0) + SLOT_GAP_CH * 2);
    expect(slotsWidthCh([])).toBe(0);
  });

  it('三档：全部 → 入 / 出 → 只出，宽度逐档变小', () => {
    const tiers = priceTiers(priceSlots({ ...tokens, cache_write_5m: null, cache_write_1h: null }));
    expect(tiers.map((t) => t.slots.map((s) => s.label))).toEqual([['入', '出', '读', '写', '写1h'], ['入', '出'], ['出']]);
    expect(tiers[0].ch).toBeGreaterThan(tiers[1].ch);
    expect(tiers[1].ch).toBeGreaterThan(tiers[2].ch);
    expect(tiers[2].ch).toBe(tiers[2].slots[0].ch);
  });

  it('五槽位比四槽位的完整档更宽；两者的入 / 出档和出档宽度相同', () => {
    const five = priceTiers(priceSlots({ ...tokens, cache_write_5m: null, cache_write_1h: null }));
    const four = priceTiers(priceSlots({ ...tokens, cache_write_5m: null }));
    expect(five[0].ch).toBeGreaterThan(four[0].ch);
    expect(five[1].ch).toBe(four[1].ch);
    expect(five[2].ch).toBe(four[2].ch);
  });

  it('各档所需宽度（ch）：五槽位 39.6、四槽位 30.4、入 / 出 14.2、出 6.6', () => {
    const five = priceTiers(priceSlots({ ...tokens, cache_write_5m: null, cache_write_1h: null }));
    const four = priceTiers(priceSlots({ ...tokens, cache_write_5m: null }));
    expect(five.map((t) => t.ch)).toEqual([39.6, 14.2, 6.6].map((n) => expect.closeTo(n, 5)));
    expect(four[0].ch).toBeCloseTo(30.4, 5);
  });

  it('按次价只有一档', () => {
    const tiers = priceTiers(priceSlots({ input: null, output: null, cache_read: null, per_request: 1 }));
    expect(tiers).toHaveLength(1);
    expect(tiers[0].slots[0].key).toBe('per_request');
  });
});

describe('priceTitle', () => {
  it('只列出有值的槽位', () => {
    const slots = priceSlots({ ...tokens, cache_read: 0.3, cache_write_5m: null });
    expect(priceTitle(slots, String)).toBe('入 3 / 出 15 / 读 0.3');
  });
});
