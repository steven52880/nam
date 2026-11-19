import { describe, expect, it } from 'vitest';
import { deviationBase, deviationHint } from './modelRatio';

describe('deviationBase', () => {
  it('优先用上游分组倍率', () => {
    expect(deviationBase({ upstream_group_ratio: 0.5, derived_median: 0.4 })).toEqual({ value: 0.5, label: '上游分组倍率' });
  });
  it('没有分组倍率时用渠道中位数', () => {
    expect(deviationBase({ upstream_group_ratio: null, derived_median: 0.4 })).toEqual({ value: 0.4, label: '渠道中位数' });
  });
  it('都没有返回 null', () => {
    expect(deviationBase({ upstream_group_ratio: null, derived_median: null })).toBeNull();
  });
});

describe('deviationHint', () => {
  const base = { value: 0.5, label: '上游分组倍率' };
  it('写清标价倍数、基准和偏离百分比', () => {
    expect(deviationHint({ derived: 0.75, deviation: 0.5 }, base)).toBe(
      '上游给这个模型的标价是原厂价的 0.75 倍，和上游分组倍率 0.5 不一致，偏离 +50.0%',
    );
  });
  it('偏离为负', () => {
    expect(deviationHint({ derived: 0.25, deviation: -0.5 }, base)).toContain('偏离 -50.0%');
  });
  it('没有基准或偏离时只说前半句', () => {
    expect(deviationHint({ derived: 0.75, deviation: null }, null)).toBe('上游给这个模型的标价是原厂价的 0.75 倍');
    expect(deviationHint({ derived: 0.75, deviation: null }, base)).toBe(
      '上游给这个模型的标价是原厂价的 0.75 倍，和上游分组倍率 0.5 不一致',
    );
  });
  it('没有推算倍率返回空串', () => {
    expect(deviationHint({ derived: null, deviation: null }, base)).toBe('');
  });
});
