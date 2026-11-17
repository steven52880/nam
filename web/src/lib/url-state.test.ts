import { describe, expect, it } from 'vitest';
import { applyUrlPatch } from './url-state';

describe('applyUrlPatch', () => {
  it('同时删除两个参数', () => {
    const prev = new URLSearchParams('batch=1&token=2&search=abc');
    const next = applyUrlPatch(prev, { batch: null, token: null });
    expect(next.has('batch')).toBe(false);
    expect(next.has('token')).toBe(false);
  });

  it('保留其它参数', () => {
    const prev = new URLSearchParams('batch=1&token=2&search=abc');
    const next = applyUrlPatch(prev, { batch: null, token: null });
    expect(next.get('search')).toBe('abc');
    expect(next.toString()).toBe('search=abc');
  });

  it('新增和修改', () => {
    const prev = new URLSearchParams('open=1');
    const next = applyUrlPatch(prev, { open: '2', search: 'x y' });
    expect(next.get('open')).toBe('2');
    expect(next.get('search')).toBe('x y');
  });

  it('不修改传入的对象；删除不存在的参数不报错', () => {
    const prev = new URLSearchParams('a=1');
    const next = applyUrlPatch(prev, { a: null, missing: null });
    expect(prev.toString()).toBe('a=1');
    expect(next.toString()).toBe('');
  });
});
