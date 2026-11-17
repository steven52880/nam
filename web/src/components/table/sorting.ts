import type { SortSpec, TableColumn } from './types';

let collator: Intl.Collator | null = null;
function getCollator(): Intl.Collator {
  if (!collator) collator = new Intl.Collator('zh-CN', { numeric: true });
  return collator;
}

type Value = string | number | null | undefined;

/** null、undefined、空字符串、NaN 视为空值。 */
function isEmpty(v: Value): boolean {
  return v === null || v === undefined || v === '' || (typeof v === 'number' && Number.isNaN(v));
}

function toNumber(v: Exclude<Value, null | undefined>): number {
  return typeof v === 'number' ? v : Number(v);
}

/**
 * 稳定排序，返回新数组。
 * - 按 specs 依次比较；找不到列的 spec 忽略（不检查 sortable，默认排序可以指定不可点击的列）。
 * - 空值不论升序降序都排在最后；两行在某个键上都为空时，继续比较下一个键。
 * - sortType 为 number 的列按数值比较（字符串先转数字，转不出来的视为空值）；其它按文字比较。
 */
export function sortRows<T>(rows: T[], specs: SortSpec[], columns: TableColumn<T>[]): T[] {
  const byId = new Map(columns.map((c) => [c.id, c]));
  const keys = specs.flatMap((spec) => {
    const col = byId.get(spec.id);
    return col ? [{ col, desc: spec.desc }] : [];
  });
  if (keys.length === 0) return rows.slice();

  const cmp = getCollator();
  const decorated = rows.map((row, index) => ({
    row,
    index,
    values: keys.map(({ col }): Value => {
      const raw = col.accessor(row);
      if (col.sortType === 'number') {
        if (isEmpty(raw)) return null;
        const n = toNumber(raw as string | number);
        return Number.isNaN(n) ? null : n;
      }
      return isEmpty(raw) ? null : String(raw);
    }),
  }));

  decorated.sort((a, b) => {
    for (let k = 0; k < keys.length; k++) {
      const va = a.values[k];
      const vb = b.values[k];
      const ea = va === null;
      const eb = vb === null;
      if (ea && eb) continue;
      if (ea) return 1;
      if (eb) return -1;
      let c: number;
      if (keys[k].col.sortType === 'number') {
        c = (va as number) - (vb as number);
      } else {
        c = cmp.compare(va as string, vb as string);
      }
      if (c !== 0) return keys[k].desc ? -c : c;
    }
    return a.index - b.index;
  });
  return decorated.map((d) => d.row);
}

/**
 * 点击表头后的用户排序。只看用户排序，不看实际生效的排序：
 * null 或其它列 → 升序；升序 → 降序；降序 → null（回到默认排序）。
 */
export function nextSort(userSort: SortSpec | null, columnId: string): SortSpec | null {
  if (!userSort || userSort.id !== columnId) return { id: columnId, desc: false };
  if (!userSort.desc) return { id: columnId, desc: true };
  return null;
}
