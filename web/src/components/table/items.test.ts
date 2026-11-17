import { describe, expect, it } from 'vitest';
import { buildItems, type BuildItemsOptions } from './items';
import type { TableItem } from './types';

interface Row {
  id: string;
  g: string;
}

const rows: Row[] = [
  { id: '1', g: 'b' },
  { id: '2', g: 'a' },
  { id: '3', g: 'b' },
  { id: '4', g: '' },
  { id: '5', g: 'a' },
];

function opts(over: Partial<BuildItemsOptions<Row>> = {}): BuildItemsOptions<Row> {
  return {
    getRowId: (r) => r.id,
    groupBy: (r) => r.g,
    expandedGroups: new Set(),
    expandAll: false,
    collapseAll: false,
    expandedRows: new Set(),
    ...over,
  };
}

function brief(items: TableItem<Row>[]): string[] {
  return items.map((it) => {
    if (it.kind === 'group') return `g:${it.key}${it.expanded ? '+' : '-'}(${it.rows.map((r) => r.id).join('')})`;
    return `${it.kind === 'row' ? 'r' : 'd'}:${it.id}`;
  });
}

describe('buildItems', () => {
  it('没有 groupBy：每行一个条目，保持顺序', () => {
    const items = buildItems(rows, opts({ groupBy: undefined }));
    expect(brief(items)).toEqual(['r:1', 'r:2', 'r:3', 'r:4', 'r:5']);
  });

  it('默认全部折叠，组按首次出现的顺序，rows 含全部行', () => {
    expect(brief(buildItems(rows, opts()))).toEqual(['g:b-(13)', 'g:a-(25)', 'g:-(4)']);
  });

  it('展开集合里的组展开，组内保持行顺序', () => {
    const items = buildItems(rows, opts({ expandedGroups: new Set(['a']) }));
    expect(brief(items)).toEqual(['g:b-(13)', 'g:a+(25)', 'r:2', 'r:5', 'g:-(4)']);
  });

  it('集合里有不存在的组键时忽略', () => {
    const items = buildItems(rows, opts({ expandedGroups: new Set(['zzz']) }));
    expect(brief(items)).toEqual(['g:b-(13)', 'g:a-(25)', 'g:-(4)']);
  });

  it('expandAll 全部展开', () => {
    const items = buildItems(rows, opts({ expandAll: true }));
    expect(brief(items)).toEqual(['g:b+(13)', 'r:1', 'r:3', 'g:a+(25)', 'r:2', 'r:5', 'g:+(4)', 'r:4']);
  });

  it('collapseAll 全部折叠，优先于 expandAll 和展开集合', () => {
    const items = buildItems(rows, opts({ expandAll: true, collapseAll: true, expandedGroups: new Set(['a']) }));
    expect(brief(items)).toEqual(['g:b-(13)', 'g:a-(25)', 'g:-(4)']);
  });

  it('展开行：row 后紧跟 detail，只对集合里的行', () => {
    const items = buildItems(rows, opts({ groupBy: undefined, expandedRows: new Set(['2', '5']) }));
    expect(brief(items)).toEqual(['r:1', 'r:2', 'd:2', 'r:3', 'r:4', 'r:5', 'd:5']);
  });

  it('分组时展开行只在组展开后出现', () => {
    const closed = buildItems(rows, opts({ expandedRows: new Set(['2']) }));
    expect(brief(closed)).toEqual(['g:b-(13)', 'g:a-(25)', 'g:-(4)']);
    const open = buildItems(rows, opts({ expandedRows: new Set(['2']), expandedGroups: new Set(['a']) }));
    expect(brief(open)).toEqual(['g:b-(13)', 'g:a+(25)', 'r:2', 'd:2', 'r:5', 'g:-(4)']);
  });

  it('sortGroups 决定组顺序', () => {
    const items = buildItems(rows, opts({ sortGroups: (keys) => [...keys].sort() }));
    expect(brief(items)).toEqual(['g:-(4)', 'g:a-(25)', 'g:b-(13)']);
  });

  it('sortGroups 收到的是出现顺序的键', () => {
    let seen: string[] = [];
    buildItems(rows, opts({ sortGroups: (keys) => ((seen = keys), keys) }));
    expect(seen).toEqual(['b', 'a', '']);
  });

  it('sortGroups 漏掉的键追加在末尾，多出来的键和重复键忽略', () => {
    const items = buildItems(rows, opts({ sortGroups: () => ['a', 'ghost', 'a'] }));
    expect(brief(items)).toEqual(['g:a-(25)', 'g:b-(13)', 'g:-(4)']);
  });

  it('空数据', () => {
    expect(buildItems([], opts())).toEqual([]);
    expect(buildItems([], opts({ groupBy: undefined }))).toEqual([]);
  });
});
