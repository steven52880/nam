import { describe, expect, it } from 'vitest';
import { isBoolean, readStored, writeStored } from './useLocalStorageState';

function fakeStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
  };
}

describe('readStored / writeStored', () => {
  it('没有存过时返回 fallback', () => {
    expect(readStored(fakeStorage(), 'k', false)).toBe(false);
    expect(readStored(null, 'k', true)).toBe(true);
  });

  it('写入后能读回', () => {
    const s = fakeStorage();
    writeStored(s, 'k', true);
    expect(s.data.get('k')).toBe('true');
    expect(readStored(s, 'k', false, isBoolean)).toBe(true);
  });

  it('坏数据、类型不对时返回 fallback', () => {
    expect(readStored(fakeStorage({ k: '{oops' }), 'k', false, isBoolean)).toBe(false);
    expect(readStored(fakeStorage({ k: '"yes"' }), 'k', false, isBoolean)).toBe(false);
  });

  it('存储抛错时不影响调用方', () => {
    const broken = {
      getItem: () => {
        throw new Error('denied');
      },
      setItem: () => {
        throw new Error('quota');
      },
    };
    expect(readStored(broken, 'k', 7)).toBe(7);
    expect(() => writeStored(broken, 'k', 1)).not.toThrow();
  });
});
