import { describe, expect, it } from 'vitest';
import { balanceFullCh, balanceParts, balanceTitle, balanceWidths } from './balance';

const row = (balance_usd: number | null, balance_cny: number | null) => ({ balance_usd, balance_cny });

describe('balanceParts / balanceTitle', () => {
  it('带货币符号，保留两位小数', () => {
    expect(balanceParts(12.3, 88.1)).toEqual({ usd: '$12.30', cny: '¥88.10' });
    expect(balanceParts(null, 5)).toEqual({ usd: null, cny: '¥5.00' });
  });

  it('提示文字：两边都有用 ≈ 连接，只有一边只写那一边', () => {
    expect(balanceTitle(balanceParts(12.3, 88.1))).toBe('$12.30 ≈ ¥88.10');
    expect(balanceTitle(balanceParts(12.3, null))).toBe('$12.30');
    expect(balanceTitle(balanceParts(null, 88.1))).toBe('¥88.10');
  });
});

describe('balanceWidths', () => {
  it('取各行两段文字的最大字符数', () => {
    // $1.00 = 5，$1234.50 = 8；¥7.20 = 5，¥12345.60 = 9
    expect(balanceWidths([row(1, 7.2), row(1234.5, 12345.6), row(null, null)])).toEqual({ usdCh: 8, cnyCh: 9 });
  });

  it('只有人民币的行，美元位置显示「—」占 1 个字符', () => {
    expect(balanceWidths([row(null, 7.2)])).toEqual({ usdCh: 1, cnyCh: 5 });
  });

  it('没有数据时为 0', () => {
    expect(balanceWidths([])).toEqual({ usdCh: 0, cnyCh: 0 });
    expect(balanceWidths([row(null, null)])).toEqual({ usdCh: 0, cnyCh: 0 });
  });
});

describe('balanceFullCh', () => {
  it('美元 + 「 ≈ 」3 个字符 + 人民币', () => {
    expect(balanceFullCh({ usdCh: 8, cnyCh: 9 })).toBe(20);
  });

  it('没有人民币时不含 ≈', () => {
    expect(balanceFullCh({ usdCh: 8, cnyCh: 0 })).toBe(8);
  });
});
