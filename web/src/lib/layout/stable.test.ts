import { describe, expect, it } from 'vitest';
import { stableStringify } from './stable';

describe('stableStringify', () => {
  it('对象键排序，键顺序不同结果相同', () => {
    expect(stableStringify({ b: 1, a: 2 })).toBe(stableStringify({ a: 2, b: 1 }));
    expect(stableStringify({ b: 1, a: 2 })).toBe('{"a":2,"b":1}');
  });

  it('嵌套对象递归排序', () => {
    const x = { z: { y: 1, x: [{ q: 1, p: 2 }] }, a: null };
    const y = { a: null, z: { x: [{ p: 2, q: 1 }], y: 1 } };
    expect(stableStringify(x)).toBe(stableStringify(y));
  });

  it('数组保持顺序', () => {
    expect(stableStringify([1, 2, 3])).not.toBe(stableStringify([3, 2, 1]));
    expect(stableStringify(['b', 'a'])).toBe('["b","a"]');
  });

  it('基本类型', () => {
    expect(stableStringify(null)).toBe('null');
    expect(stableStringify('a"b')).toBe('"a\\"b"');
    expect(stableStringify(1.5)).toBe('1.5');
    expect(stableStringify(true)).toBe('true');
    expect(stableStringify(NaN)).toBe('null');
  });

  it('undefined：对象里跳过，数组里写 null，顶层写 null', () => {
    expect(stableStringify({ a: undefined, b: 1 })).toBe('{"b":1}');
    expect(stableStringify([undefined, 1])).toBe('[null,1]');
    expect(stableStringify(undefined)).toBe('null');
  });

  it('值不同则结果不同', () => {
    expect(stableStringify({ a: 1 })).not.toBe(stableStringify({ a: 2 }));
    expect(stableStringify({ a: 1 })).not.toBe(stableStringify({ b: 1 }));
    expect(stableStringify({ a: '1' })).not.toBe(stableStringify({ a: 1 }));
  });
});
