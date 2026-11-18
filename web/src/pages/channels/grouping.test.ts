import { describe, expect, it } from 'vitest';
import { buildGroupLookups, groupKeyOf, groupLabelOf, sortGroupKeys } from './grouping';
import { makeChannel } from './testRows';

describe('groupKeyOf', () => {
  it('按上游：upstream_id 的字符串，没有为空串', () => {
    const key = groupKeyOf('upstream')!;
    expect(key(makeChannel({ upstream_id: 7 }))).toBe('7');
    expect(key(makeChannel({ upstream_id: null, upstream_name: '' }))).toBe('');
  });

  it('按厂商、标签：取原值，没有为空串', () => {
    expect(groupKeyOf('vendor')!(makeChannel({ vendor: 'openai' }))).toBe('openai');
    expect(groupKeyOf('vendor')!(makeChannel({ vendor: '' }))).toBe('');
    expect(groupKeyOf('tag')!(makeChannel({ tag: 'a' }))).toBe('a');
    expect(groupKeyOf('tag')!(makeChannel({ tag: '' }))).toBe('');
  });

  it('不分组返回 undefined', () => {
    expect(groupKeyOf('none')).toBeUndefined();
  });

  it('上游改名不影响组键', () => {
    const key = groupKeyOf('upstream')!;
    expect(key(makeChannel({ upstream_id: 3, upstream_name: '旧名' }))).toBe(key(makeChannel({ upstream_id: 3, upstream_name: '新名' })));
  });
});

describe('sortGroupKeys', () => {
  const label = (k: string) => k;
  const ctx = (over: Partial<Parameters<typeof sortGroupKeys>[2]> = {}) => ({
    vendorOrder: [],
    savedOrder: [],
    labelOf: label,
    ...over,
  });

  it('按厂商：vendorOrder 里的按顺序', () => {
    expect(sortGroupKeys('vendor', ['c', 'a', 'b'], ctx({ vendorOrder: ['b', 'c', 'a'] }))).toEqual(['b', 'c', 'a']);
  });

  it('按厂商：不在 vendorOrder 里的按名称排在后面', () => {
    expect(sortGroupKeys('vendor', ['zeta', 'known', 'alpha'], ctx({ vendorOrder: ['known'] }))).toEqual(['known', 'alpha', 'zeta']);
  });

  it('按厂商：vendorOrder 里有但没有渠道的厂商被忽略，不影响其余顺序', () => {
    expect(sortGroupKeys('vendor', ['b', 'a'], ctx({ vendorOrder: ['x', 'a', 'y', 'b'] }))).toEqual(['a', 'b']);
  });

  it('兜底组 \'\' 永远最后（即使在保存的顺序里）', () => {
    expect(sortGroupKeys('vendor', ['', 'a', 'b'], ctx({ vendorOrder: ['a'] }))).toEqual(['a', 'b', '']);
    expect(sortGroupKeys('tag', ['', 'a'], ctx({ savedOrder: ['', 'a'] }))).toEqual(['a', '']);
    expect(sortGroupKeys('upstream', ['', '2', '1'], ctx())).toEqual(['1', '2', '']);
  });

  it('按上游：保存的顺序在前，新上游按名称排在后面', () => {
    const names: Record<string, string> = { '1': '乙', '2': '甲', '3': '丙', '4': '丁' };
    const labelOf = (k: string) => names[k] ?? k;
    expect(sortGroupKeys('upstream', ['4', '3', '2', '1'], ctx({ savedOrder: ['2', '1'], labelOf }))).toEqual(['2', '1', '3', '4']);
  });

  it('按上游：上游改名不影响保存的顺序（键是 upstream_id）', () => {
    const before = sortGroupKeys('upstream', ['1', '2'], ctx({ savedOrder: ['2', '1'], labelOf: (k) => (k === '1' ? '旧名' : '乙') }));
    const after = sortGroupKeys('upstream', ['1', '2'], ctx({ savedOrder: ['2', '1'], labelOf: (k) => (k === '1' ? 'Z新名' : '乙') }));
    expect(before).toEqual(['2', '1']);
    expect(after).toEqual(before);
  });

  it('按标签：保存的顺序加新标签', () => {
    expect(sortGroupKeys('tag', ['new', 'b', 'a'], ctx({ savedOrder: ['b', 'a'] }))).toEqual(['b', 'a', 'new']);
  });

  it('不改动传入数组；不分组保持原样', () => {
    const keys = ['b', 'a'];
    sortGroupKeys('tag', keys, ctx());
    expect(keys).toEqual(['b', 'a']);
    expect(sortGroupKeys('none', keys, ctx())).toEqual(['b', 'a']);
  });
});

describe('groupLabelOf', () => {
  const lookups = buildGroupLookups([
    makeChannel({ id: 1, upstream_id: 1, upstream_name: '上游甲' }),
    makeChannel({ id: 2, upstream_id: 2, upstream_name: '上游乙' }),
    makeChannel({ id: 3, upstream_id: null, upstream_name: '' }),
  ]);

  it('兜底组文字', () => {
    expect(groupLabelOf('upstream', '', lookups)).toBe('未托管');
    expect(groupLabelOf('vendor', '', lookups)).toBe('无厂商');
    expect(groupLabelOf('tag', '', lookups)).toBe('无标签');
  });

  it('上游名来自 upstream_id → upstream_name 映射', () => {
    expect(groupLabelOf('upstream', '1', lookups)).toBe('上游甲');
    expect(groupLabelOf('upstream', '2', lookups)).toBe('上游乙');
  });

  it('映射里没有的上游给出可读的兜底文字', () => {
    expect(groupLabelOf('upstream', '99', lookups)).toBe('上游 #99');
  });

  it('厂商、标签直接显示原值', () => {
    expect(groupLabelOf('vendor', 'openai', lookups)).toBe('openai');
    expect(groupLabelOf('tag', 'fast', lookups)).toBe('fast');
  });

  it('上游改名后标题跟着变', () => {
    const renamed = buildGroupLookups([makeChannel({ upstream_id: 1, upstream_name: '新名' })]);
    expect(groupLabelOf('upstream', '1', renamed)).toBe('新名');
  });
});
