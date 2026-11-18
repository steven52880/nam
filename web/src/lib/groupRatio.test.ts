import { describe, expect, it } from 'vitest';
import { DASH, formatGroupRatio, roundGroupRatio } from './format';

describe('roundGroupRatio / formatGroupRatio', () => {
  it('四舍五入到 3 位小数', () => {
    expect(roundGroupRatio(0.30857)).toBe(0.309);
    expect(roundGroupRatio(0.3084)).toBe(0.308);
    expect(roundGroupRatio(1.2)).toBe(1.2);
    expect(formatGroupRatio(0.30857)).toBe('0.309');
    expect(formatGroupRatio(0.285714285)).toBe('0.286');
  });
  it('去掉多余的 0', () => {
    expect(formatGroupRatio(1)).toBe('1');
    expect(formatGroupRatio(0.5)).toBe('0.5');
    expect(formatGroupRatio(0.3004)).toBe('0.3');
    expect(formatGroupRatio(12)).toBe('12');
  });
  it('NewAPI 里位数很多的值也按 3 位显示', () => {
    expect(formatGroupRatio(0.308571428571)).toBe('0.309');
    expect(formatGroupRatio(1.00000001)).toBe('1');
  });
  it('空值显示 —', () => {
    expect(formatGroupRatio(null)).toBe(DASH);
    expect(formatGroupRatio(undefined)).toBe(DASH);
  });
});
