import { describe, expect, it } from 'vitest';
import { DASH, formatRatioPercent } from './format';
import { DEFAULT_USD_CNY_RATE, MAX_USD_CNY_RATE, channelPrice, isUsdCnyRateValid, parseUsdCnyRate } from './usdRate';

describe('formatRatioPercent', () => {
  it('原厂价倍数显示成百分比，保留 3 位有效数字', () => {
    expect(formatRatioPercent(0.024)).toBe('2.4%');
    expect(formatRatioPercent(0.0085)).toBe('0.85%');
    expect(formatRatioPercent(0.35 * 0.5 / 7.2)).toBe('2.43%');
    expect(formatRatioPercent(1)).toBe('100%');
    expect(formatRatioPercent(1.5)).toBe('150%');
    expect(formatRatioPercent(12.345)).toBe('1230%');
    expect(formatRatioPercent(0.000123)).toBe('0.0123%');
  });

  it('0 显示 0%，空值显示 —', () => {
    expect(formatRatioPercent(0)).toBe('0%');
    expect(formatRatioPercent(null)).toBe(DASH);
    expect(formatRatioPercent(undefined)).toBe(DASH);
  });

  it('digits 控制有效数字位数', () => {
    expect(formatRatioPercent(0.02437, 2)).toBe('2.4%');
    expect(formatRatioPercent(0.02437, 4)).toBe('2.437%');
  });
});

describe('parseUsdCnyRate / channelPrice', () => {
  it('取到合法值就用，否则回到默认 7.2', () => {
    expect(parseUsdCnyRate('7.3')).toBe(7.3);
    expect(parseUsdCnyRate(' 6.9 ')).toBe(6.9);
    expect(parseUsdCnyRate(undefined)).toBe(7.2);
    expect(parseUsdCnyRate(null)).toBe(7.2);
    expect(parseUsdCnyRate('')).toBe(7.2);
    expect(parseUsdCnyRate('0')).toBe(7.2);
    expect(parseUsdCnyRate('abc')).toBe(7.2);
    expect(parseUsdCnyRate('101')).toBe(7.2);
  });

  it('上游渠道售价 = 分组倍率 × 汇率 ÷ 参考汇率，显示为百分比', () => {
    expect(formatRatioPercent(channelPrice(0.35, 0.5, 7.2))).toBe('2.43%');
    expect(formatRatioPercent(channelPrice(1, 7.2, 7.2))).toBe('100%');
  });
});

describe('isUsdCnyRateValid', () => {
  it('正数且不超过 100 通过', () => {
    expect(DEFAULT_USD_CNY_RATE).toBe('7.2');
    expect(MAX_USD_CNY_RATE).toBe(100);
    expect(isUsdCnyRateValid('7.2')).toBe(true);
    expect(isUsdCnyRateValid(' 7 ')).toBe(true);
    expect(isUsdCnyRateValid('0.01')).toBe(true);
    expect(isUsdCnyRateValid('100')).toBe(true);
  });

  it('空、0、负数、超过 100、非数字都不通过', () => {
    expect(isUsdCnyRateValid('')).toBe(false);
    expect(isUsdCnyRateValid('0')).toBe(false);
    expect(isUsdCnyRateValid('-7')).toBe(false);
    expect(isUsdCnyRateValid('100.01')).toBe(false);
    expect(isUsdCnyRateValid('abc')).toBe(false);
    expect(isUsdCnyRateValid('7.2.1')).toBe(false);
    expect(isUsdCnyRateValid('Infinity')).toBe(false);
    expect(isUsdCnyRateValid('1e2')).toBe(false);
  });
});
