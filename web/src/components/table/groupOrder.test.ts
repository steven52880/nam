import { describe, expect, it } from 'vitest';
import { mergeVisibleOrder } from './groupOrder';

describe('mergeVisibleOrder', () => {
  it('全部可见：直接采用新顺序', () => {
    expect(mergeVisibleOrder(['A', 'B', 'C'], ['C', 'A', 'B'])).toEqual(['C', 'A', 'B']);
  });

  it('还没保存过顺序，A/B/C 只见 A/C，拖成 C/A → C/B/A', () => {
    expect(mergeVisibleOrder(['A', 'B', 'C'], ['C', 'A'])).toEqual(['C', 'B', 'A']);
  });

  it('筛选后拖动不丢失其它组的位置', () => {
    expect(mergeVisibleOrder(['A', 'X', 'B', 'Y', 'C', 'Z'], ['C', 'A', 'B'])).toEqual(['C', 'X', 'A', 'Y', 'B', 'Z']);
  });

  it('被筛掉的组位置不变（在开头和结尾）', () => {
    expect(mergeVisibleOrder(['X', 'A', 'B', 'Y'], ['B', 'A'])).toEqual(['X', 'B', 'A', 'Y']);
  });

  it('可见组顺序没变时结果等于原顺序', () => {
    expect(mergeVisibleOrder(['A', 'B', 'C'], ['A', 'C'])).toEqual(['A', 'B', 'C']);
  });

  it('新组（fullOrder 里没有）追加到末尾', () => {
    expect(mergeVisibleOrder(['A', 'B'], ['N', 'A'])).toEqual(['N', 'B', 'A']);
    expect(mergeVisibleOrder(['A', 'B'], ['A', 'B', 'N'])).toEqual(['A', 'B', 'N']);
  });

  it('兜底组（空字符串）当作普通键', () => {
    expect(mergeVisibleOrder(['A', '', 'B'], ['B', ''])).toEqual(['A', 'B', '']);
  });

  it('空输入', () => {
    expect(mergeVisibleOrder([], [])).toEqual([]);
    expect(mergeVisibleOrder(['A', 'B'], [])).toEqual(['A', 'B']);
    expect(mergeVisibleOrder([], ['A', 'B'])).toEqual(['A', 'B']);
  });

  it('不修改入参', () => {
    const full = ['A', 'B', 'C'];
    const vis = ['C', 'A'];
    mergeVisibleOrder(full, vis);
    expect(full).toEqual(['A', 'B', 'C']);
    expect(vis).toEqual(['C', 'A']);
  });
});
