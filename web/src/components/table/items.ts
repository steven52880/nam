import type { TableItem } from './types';

export interface BuildItemsOptions<T> {
  getRowId: (row: T) => string;
  groupBy?: (row: T) => string;
  sortGroups?: (keys: string[]) => string[];
  expandedGroups: ReadonlySet<string>;
  /** 为 true 时组默认展开，collapsedGroups 里的组才折叠；此时不看 expandedGroups。默认 false。 */
  defaultExpanded?: boolean;
  /** 仅 defaultExpanded 为 true 时使用：被收起的组。 */
  collapsedGroups?: ReadonlySet<string>;
  expandAll: boolean;
  /** 拖动分组时为 true；优先于 expandAll。 */
  collapseAll: boolean;
  /** renderExpanded 用：已展开详情的行 id。 */
  expandedRows: ReadonlySet<string>;
}

/**
 * 把排好序的行扁平化成渲染用的条目。
 * - 没有 groupBy：每行一个 row 条目，展开的行后面跟一个 detail 条目。
 * - 有 groupBy：每组一个 group 条目（rows 含该组全部行），只有展开的组才输出它的行。
 *   组内保持传入行的顺序；组的顺序默认是组键第一次出现的顺序，提供 sortGroups 时以它为准
 *   （它漏掉的键按出现顺序追加在末尾，多出来的键忽略）。兜底组（键为空字符串）不做特殊处理。
 * - 组是否展开：collapseAll 为 true 时一律折叠；否则 expandAll 展开；否则默认折叠时看 expandedGroups，
 *   默认展开（defaultExpanded）时看 collapsedGroups。
 */
export function buildItems<T>(rows: T[], opts: BuildItemsOptions<T>): TableItem<T>[] {
  const { getRowId, groupBy, sortGroups, expandedGroups, defaultExpanded, collapsedGroups, expandAll, collapseAll, expandedRows } = opts;
  const items: TableItem<T>[] = [];

  const pushRow = (row: T) => {
    const id = getRowId(row);
    items.push({ kind: 'row', id, row });
    if (expandedRows.has(id)) items.push({ kind: 'detail', id, row });
  };

  if (!groupBy) {
    for (const row of rows) pushRow(row);
    return items;
  }

  const groups = new Map<string, T[]>();
  for (const row of rows) {
    const key = groupBy(row);
    const list = groups.get(key);
    if (list) list.push(row);
    else groups.set(key, [row]);
  }

  let keys = [...groups.keys()];
  if (sortGroups) {
    const sorted = sortGroups(keys.slice());
    const seen = new Set<string>();
    const ordered: string[] = [];
    for (const k of sorted) {
      if (groups.has(k) && !seen.has(k)) {
        seen.add(k);
        ordered.push(k);
      }
    }
    for (const k of keys) if (!seen.has(k)) ordered.push(k);
    keys = ordered;
  }

  for (const key of keys) {
    const groupRows = groups.get(key) ?? [];
    const open = defaultExpanded ? !collapsedGroups?.has(key) : expandedGroups.has(key);
    const expanded = !collapseAll && (expandAll || open);
    items.push({ kind: 'group', key, rows: groupRows, expanded });
    if (expanded) for (const row of groupRows) pushRow(row);
  }
  return items;
}
