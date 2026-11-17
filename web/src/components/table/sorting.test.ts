import { describe, expect, it } from 'vitest';
import { numberCol, textCol } from './columns';
import { nextSort, sortRows } from './sorting';

interface R {
  id: string;
  name: string | null;
  n: number | null;
  s?: string;
}

const cols = [
  textCol<R>('name', '名称', (r) => r.name),
  numberCol<R>('n', '数值', (r) => r.n),
  textCol<R>('s', '字符串', (r) => r.s),
];

const ids = (rows: R[]) => rows.map((r) => r.id).join(',');
const mk = (id: string, name: string | null, n: number | null, s?: string): R => ({ id, name, n, s });

describe('sortRows', () => {
  it('数字按数值比较，不是文字（10 > 9）', () => {
    const rows = [mk('a', null, 10), mk('b', null, 9), mk('c', null, 100)];
    expect(ids(sortRows(rows, [{ id: 'n', desc: false }], cols))).toBe('b,a,c');
    expect(ids(sortRows(rows, [{ id: 'n', desc: true }], cols))).toBe('c,a,b');
  });

  it('空值排最后：升序和降序都是', () => {
    const rows = [mk('a', null, null), mk('b', null, 2), mk('c', null, 1), mk('d', null, null)];
    expect(ids(sortRows(rows, [{ id: 'n', desc: false }], cols))).toBe('c,b,a,d');
    expect(ids(sortRows(rows, [{ id: 'n', desc: true }], cols))).toBe('b,c,a,d');
  });

  it('null、undefined、空字符串、NaN 都算空值', () => {
    const rows = [
      mk('a', '', 0, undefined),
      mk('b', 'x', NaN, 'y'),
      mk('c', null, 5, 'a'),
      mk('d', 'a', 1, 'b'),
    ];
    expect(ids(sortRows(rows, [{ id: 'name', desc: false }], cols))).toBe('d,b,a,c');
    expect(ids(sortRows(rows, [{ id: 'name', desc: true }], cols))).toBe('b,d,a,c');
    expect(ids(sortRows(rows, [{ id: 'n', desc: false }], cols))).toBe('a,d,c,b');
    expect(ids(sortRows(rows, [{ id: 's', desc: false }], cols))).toBe('c,d,b,a');
  });

  it('0 不是空值', () => {
    const rows = [mk('a', null, null), mk('b', null, 0), mk('c', null, -1)];
    expect(ids(sortRows(rows, [{ id: 'n', desc: false }], cols))).toBe('c,b,a');
  });

  it('两行在某个键上都为空时，继续比较下一个键', () => {
    const rows = [mk('a', null, 3), mk('b', null, 1), mk('c', 'z', 9), mk('d', null, 2)];
    const asc = sortRows(rows, [{ id: 'name', desc: false }, { id: 'n', desc: false }], cols);
    expect(ids(asc)).toBe('c,b,d,a');
    const desc = sortRows(rows, [{ id: 'name', desc: false }, { id: 'n', desc: true }], cols);
    expect(ids(desc)).toBe('c,a,d,b');
  });

  it('第一个键相等时用第二个键，第二个键的方向独立', () => {
    const rows = [mk('a', 'x', 1), mk('b', 'y', 1), mk('c', 'x', 2), mk('d', 'y', 2)];
    expect(ids(sortRows(rows, [{ id: 'name', desc: false }, { id: 'n', desc: true }], cols))).toBe('c,a,d,b');
  });

  it('稳定：完全相等时保持原顺序（升序和降序都是）', () => {
    const rows = [mk('a', null, 1), mk('b', null, 1), mk('c', null, 1), mk('d', null, null), mk('e', null, null)];
    expect(ids(sortRows(rows, [{ id: 'n', desc: false }], cols))).toBe('a,b,c,d,e');
    expect(ids(sortRows(rows, [{ id: 'n', desc: true }], cols))).toBe('a,b,c,d,e');
  });

  it('中文排序（拼音）与数字自然排序', () => {
    const rows = [mk('a', '张三', null), mk('b', '阿里', null), mk('c', '李四', null), mk('d', '王五', null)];
    expect(ids(sortRows(rows, [{ id: 'name', desc: false }], cols))).toBe('b,c,d,a');
    const nat = [mk('a', 'gpt-10', null), mk('b', 'gpt-2', null), mk('c', 'gpt-1', null)];
    expect(ids(sortRows(nat, [{ id: 'name', desc: false }], cols))).toBe('c,b,a');
  });

  it('不修改入参；没有排序键时返回拷贝', () => {
    const rows = [mk('b', 'b', 2), mk('a', 'a', 1)];
    const sorted = sortRows(rows, [{ id: 'name', desc: false }], cols);
    expect(ids(rows)).toBe('b,a');
    expect(ids(sorted)).toBe('a,b');
    const same = sortRows(rows, [], cols);
    expect(same).not.toBe(rows);
    expect(ids(same)).toBe('b,a');
  });

  it('找不到列的排序键忽略', () => {
    const rows = [mk('b', 'b', 2), mk('a', 'a', 1)];
    expect(ids(sortRows(rows, [{ id: 'nope', desc: false }, { id: 'name', desc: false }], cols))).toBe('a,b');
  });

  it('sortType 为 number 的列，字符串数字按数值比较', () => {
    const strCols = [numberCol<{ id: string; v: string }>('v', 'v', (r) => r.v)];
    const rows = [
      { id: 'a', v: '10' },
      { id: 'b', v: '9' },
      { id: 'c', v: 'abc' },
    ];
    expect(sortRows(rows, [{ id: 'v', desc: false }], strCols).map((r) => r.id).join(',')).toBe('b,a,c');
  });
});

describe('nextSort', () => {
  it('null → 该列升序', () => {
    expect(nextSort(null, 'a')).toEqual({ id: 'a', desc: false });
  });

  it('升序 → 降序 → null', () => {
    expect(nextSort({ id: 'a', desc: false }, 'a')).toEqual({ id: 'a', desc: true });
    expect(nextSort({ id: 'a', desc: true }, 'a')).toBeNull();
  });

  it('其它列的用户排序 → 新列升序（即使原来是降序）', () => {
    expect(nextSort({ id: 'b', desc: true }, 'a')).toEqual({ id: 'a', desc: false });
  });

  it('只看用户排序：默认排序含该列时，点击仍先升序', () => {
    const defaultSort = [{ id: 'priority', desc: true }];
    // 用户还没排序过（userSort = null），默认排序是 priority 降序，点击 priority 应得升序
    expect(defaultSort[0].desc).toBe(true);
    expect(nextSort(null, 'priority')).toEqual({ id: 'priority', desc: false });
  });
});
