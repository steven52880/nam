import { describe, expect, it } from 'vitest';
import { buildGroupLookups, groupKeyOf, groupLabelOf } from './grouping';
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
