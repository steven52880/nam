import { describe, expect, it } from 'vitest';
import { customCol, numberCol, pickColumns, textCol } from './columns';
import {
  DEFAULT_MIN_REM,
  DEFAULT_WIDTH_REM,
  MAX_WIDTH_REM,
  defaultTableLayout,
  diffVisibility,
  diffWidths,
  effectiveVisibility,
  parseTableLayout,
  reconcileOrder,
  reconcileWidths,
} from './layout';
import type { TableColumn } from './types';
import { remToPx } from '@/lib/size';

type R = { id: string };
const col = (id: string, o: Partial<TableColumn<R>> = {}): TableColumn<R> => ({
  id,
  header: id,
  accessor: () => null,
  ...o,
});

describe('reconcileOrder', () => {
  it('没有保存过：沿用默认顺序', () => {
    expect(reconcileOrder([], ['a', 'b', 'c'], ['a', 'b', 'c'])).toEqual(['a', 'b', 'c']);
  });

  it('保存的顺序保持不变', () => {
    expect(reconcileOrder(['c', 'a', 'b'], ['a', 'b', 'c'], ['a', 'b', 'c'])).toEqual(['c', 'a', 'b']);
  });

  it('删除列：保存里不存在的 id 被去掉，重复 id 只留一个', () => {
    expect(reconcileOrder(['c', 'x', 'a', 'c'], ['a', 'c'], ['a', 'c'])).toEqual(['c', 'a']);
  });

  it('新增列：插在它在默认顺序里的前一列后面', () => {
    expect(reconcileOrder(['c', 'a'], ['a', 'b', 'c'], ['a', 'b', 'c'])).toEqual(['c', 'a', 'b']);
    expect(reconcileOrder(['c', 'a', 'b'], ['a', 'b', 'n', 'c'], ['a', 'b', 'n', 'c'])).toEqual(['c', 'a', 'b', 'n']);
  });

  it('新增列：默认顺序里排第一时放最前', () => {
    expect(reconcileOrder(['b', 'a'], ['n', 'a', 'b'], ['n', 'a', 'b'])).toEqual(['n', 'b', 'a']);
  });

  it('新增列：前一列也是新列时，按顺序依次插入', () => {
    expect(reconcileOrder(['c', 'a'], ['a', 'n1', 'n2', 'c'], ['a', 'n1', 'n2', 'c'])).toEqual(['c', 'a', 'n1', 'n2']);
  });

  it('新增列：前一列已被删除时，继续往前找', () => {
    // 默认 a, gone(已不存在), n：n 前面最近的存在的列是 a
    expect(reconcileOrder(['b', 'a'], ['a', 'b', 'n'], ['a', 'gone', 'n', 'b'])).toEqual(['b', 'a', 'n']);
  });

  it('不在默认顺序里的新列追加到末尾', () => {
    expect(reconcileOrder(['a'], ['a', 'z'], ['a'])).toEqual(['a', 'z']);
  });

  it('保存为空且列全是新列：结果等于默认顺序', () => {
    expect(reconcileOrder([], ['b', 'a'], ['a', 'b'])).toEqual(['a', 'b']);
  });

  it('不修改入参', () => {
    const saved = ['b', 'a'];
    reconcileOrder(saved, ['a', 'b', 'c'], ['a', 'b', 'c']);
    expect(saved).toEqual(['b', 'a']);
  });
});

describe('reconcileWidths', () => {
  it('没保存的列用默认宽度（不传 width 为 6.25rem）', () => {
    const w = reconcileWidths({}, [col('a'), col('b', { width: 10 })]);
    expect(w.a).toBe(remToPx(DEFAULT_WIDTH_REM));
    expect(w.b).toBe(remToPx(10));
  });

  it('保存的宽度从 rem 换算成像素', () => {
    const w = reconcileWidths({ a: 12 }, [col('a', { width: 10 })]);
    expect(w.a).toBe(remToPx(12));
  });

  it('下限：默认 3rem，列可以自定义 minWidth', () => {
    const w = reconcileWidths({ a: 1, b: 1 }, [col('a'), col('b', { minWidth: 5 })]);
    expect(w.a).toBe(remToPx(DEFAULT_MIN_REM));
    expect(w.b).toBe(remToPx(5));
  });

  it('上限 60rem', () => {
    const w = reconcileWidths({ a: 999 }, [col('a')]);
    expect(w.a).toBe(remToPx(MAX_WIDTH_REM));
  });

  it('默认宽度也受限制', () => {
    const w = reconcileWidths({}, [col('a', { width: 1 }), col('b', { width: 100 })]);
    expect(w.a).toBe(remToPx(DEFAULT_MIN_REM));
    expect(w.b).toBe(remToPx(MAX_WIDTH_REM));
  });

  it('已删除的列的保存值被忽略；非法数值回退默认', () => {
    const w = reconcileWidths({ gone: 20, a: NaN }, [col('a')]);
    expect(Object.keys(w)).toEqual(['a']);
    expect(w.a).toBe(remToPx(DEFAULT_WIDTH_REM));
  });

  it('fixed 列也有宽度', () => {
    const w = reconcileWidths({}, [col('__select', { fixed: true, width: 2.5, minWidth: 2.5 })]);
    expect(w.__select).toBe(remToPx(2.5));
  });
});

describe('diffWidths', () => {
  it('只记与默认不同的列，取到 0.25rem', () => {
    const cols = [col('a', { width: 10 }), col('b', { width: 10 }), col('c')];
    const r = diffWidths({ a: remToPx(10), b: remToPx(12.1), c: remToPx(8) }, cols);
    expect(r).toEqual({ b: 12, c: 8 });
  });

  it('fixed 列和不存在的列不记录，超限的值被限制', () => {
    const cols = [col('f', { fixed: true }), col('a')];
    expect(diffWidths({ f: remToPx(9), a: remToPx(100), gone: 5 }, cols)).toEqual({ a: MAX_WIDTH_REM });
  });

  it('对账后的宽度再 diff 不产生任何记录（不产生未保存）', () => {
    const cols = [col('a', { width: 10 }), col('b')];
    expect(diffWidths(reconcileWidths({}, cols), cols)).toEqual({});
  });

  it('ceilIds 里的列向上取到 0.25rem，其它列仍四舍五入', () => {
    const cols = [col('a', { width: 10 }), col('b', { width: 10 })];
    const px = { a: remToPx(12.1), b: remToPx(12.1) };
    expect(diffWidths(px, cols)).toEqual({ a: 12, b: 12 });
    expect(diffWidths(px, cols, { ceilIds: new Set(['a']) })).toEqual({ a: 12.25, b: 12 });
  });

  it('向上取：保存后的宽度不小于测量值，且重复保存结果不变', () => {
    const cols = [col('a', { width: 10 })];
    const measured = Math.ceil(remToPx(12.3));
    const first = diffWidths({ a: measured }, cols, { ceilIds: new Set(['a']) }).a;
    expect(remToPx(first)).toBeGreaterThanOrEqual(measured);
    // 用保存后的宽度换算回像素，再按"自动调整"保存一次：结果不变
    expect(diffWidths({ a: remToPx(first) }, cols, { ceilIds: new Set(['a']) }).a).toBe(first);
  });

  it('向上取：刚好是 0.25 的倍数时不被浮点噪声抬高', () => {
    const cols = [col('a', { width: 10 })];
    expect(diffWidths({ a: remToPx(13.75) + 1e-9 }, cols, { ceilIds: new Set(['a']) })).toEqual({ a: 13.75 });
  });

  it('向上取：不超过 60rem，不低于最小列宽，等于默认宽度时不记录', () => {
    const cols = [col('a', { width: 10 }), col('b', { minWidth: 5 }), col('c', { width: 10 })];
    const ceilIds = new Set(['a', 'b', 'c']);
    expect(diffWidths({ a: remToPx(59.9) }, cols, { ceilIds }).a).toBe(MAX_WIDTH_REM);
    expect(diffWidths({ a: remToPx(80) }, cols, { ceilIds }).a).toBe(MAX_WIDTH_REM);
    expect(diffWidths({ b: remToPx(1) }, cols, { ceilIds }).b).toBe(5);
    expect(diffWidths({ c: remToPx(9.9) }, cols, { ceilIds })).toEqual({});
  });
});

describe('diffVisibility / effectiveVisibility', () => {
  const cols = [col('a'), col('b', { hidden: true }), col('f', { fixed: true })];

  it('只返回与默认不同的项', () => {
    expect(diffVisibility({ a: true, b: false, f: true }, cols)).toEqual({});
    expect(diffVisibility({ a: false, b: true }, cols)).toEqual({ a: false, b: true });
  });

  it('fixed 列永远可见，不记录', () => {
    expect(diffVisibility({ f: false }, cols)).toEqual({});
    expect(effectiveVisibility({ f: false }, cols).f).toBe(true);
  });

  it('不存在的列和非布尔值不记录', () => {
    expect(diffVisibility({ gone: false } as Record<string, boolean>, cols)).toEqual({});
  });

  it('生效可见性：保存的覆盖默认；没保存的取默认（hidden 取反）', () => {
    expect(effectiveVisibility({}, cols)).toEqual({ a: true, b: false, f: true });
    expect(effectiveVisibility({ a: false, b: true }, cols)).toEqual({ a: false, b: true, f: true });
  });

  it('新增的默认隐藏列不会自己显示出来', () => {
    expect(effectiveVisibility({ a: true }, [...cols, col('n', { hidden: true })]).n).toBe(false);
  });
});

describe('parseTableLayout', () => {
  it('合法数据原样通过（返回新对象）', () => {
    const data = { widths: { a: 8 }, order: ['b', 'a'], visibility: { a: false }, sort: { id: 'a', desc: true } };
    const r = parseTableLayout(data);
    expect(r).toEqual(data);
    expect(r).not.toBe(data);
  });

  it('缺少的字段取默认；sort 为 null 通过', () => {
    expect(parseTableLayout({})).toEqual(defaultTableLayout());
    expect(parseTableLayout({ sort: null })).toEqual(defaultTableLayout());
  });

  it('不是对象返回 null', () => {
    for (const v of [null, undefined, 1, 'x', [], true]) expect(parseTableLayout(v)).toBeNull();
  });

  it('字段类型不对返回 null', () => {
    expect(parseTableLayout({ widths: [] })).toBeNull();
    expect(parseTableLayout({ widths: { a: '8' } })).toBeNull();
    expect(parseTableLayout({ widths: { a: -1 } })).toBeNull();
    expect(parseTableLayout({ widths: { a: NaN } })).toBeNull();
    expect(parseTableLayout({ order: 'a' })).toBeNull();
    expect(parseTableLayout({ order: ['a', 1] })).toBeNull();
    expect(parseTableLayout({ visibility: { a: 1 } })).toBeNull();
    expect(parseTableLayout({ sort: 'a' })).toBeNull();
    expect(parseTableLayout({ sort: { id: 'a' } })).toBeNull();
    expect(parseTableLayout({ sort: { id: 1, desc: false } })).toBeNull();
  });

  it('defaultTableLayout 每次返回新对象', () => {
    const a = defaultTableLayout();
    expect(a).toEqual({ widths: {}, order: [], visibility: {}, sort: null });
    expect(defaultTableLayout()).not.toBe(a);
  });
});

describe('列构造函数', () => {
  it('textCol：左对齐，text 排序', () => {
    const c = textCol<R>('a', 'A', (r) => r.id, { width: 8 });
    expect(c).toMatchObject({ id: 'a', header: 'A', align: 'left', sortType: 'text', width: 8 });
    expect(c.accessor({ id: 'x' })).toBe('x');
  });

  it('numberCol：右对齐、number 排序、tabular-nums，并保留调用方的 className', () => {
    const c = numberCol<R>('n', 'N', () => 1);
    expect(c).toMatchObject({ align: 'right', sortType: 'number', className: 'tabular-nums' });
    expect(numberCol<R>('n', 'N', () => 1, { className: 'x' }).className).toBe('tabular-nums x');
    expect(numberCol<R>('n', 'N', () => 1, { align: 'left' }).align).toBe('left');
  });

  it('customCol：没有 accessor 时不可排序', () => {
    const c = customCol<R>('c', 'C', { cell: () => null, fixed: true });
    expect(c.sortable).toBe(false);
    expect(c.accessor({ id: 'x' })).toBeNull();
    expect(c.fixed).toBe(true);
    expect(customCol<R>('c', 'C', { accessor: (r) => r.id }).sortable).toBeUndefined();
  });

  it('pickColumns：按 ids 顺序挑选，忽略不存在的和重复的', () => {
    const all = [col('a'), col('b'), col('c')];
    expect(pickColumns(all, ['c', 'x', 'a', 'c']).map((c) => c.id)).toEqual(['c', 'a']);
  });
});
