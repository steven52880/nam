import type { TableColumn } from './types';

type Accessor<T> = TableColumn<T>['accessor'];
type ColOpts<T> = Partial<Omit<TableColumn<T>, 'id' | 'header' | 'accessor'>>;

/** 合成列：勾选、拖动手柄。 */
export const SELECT_COLUMN_ID = '__select';
export const DRAG_COLUMN_ID = '__drag';

/** 数字列使用等宽数字，渲染方把 `TableColumn.className` 挂到 data-cell-content 上。 */
export const NUMBER_CLASS = 'tabular-nums';

function joinClass(...parts: Array<string | undefined>): string | undefined {
  const s = parts.filter(Boolean).join(' ');
  return s === '' ? undefined : s;
}

/** 文字列：左对齐，按文字排序。 */
export function textCol<T>(id: string, header: string, accessor: Accessor<T>, opts: ColOpts<T> = {}): TableColumn<T> {
  return { align: 'left', sortType: 'text', ...opts, id, header, accessor };
}

/** 数字列：右对齐 + 等宽数字，按数值排序。 */
export function numberCol<T>(id: string, header: string, accessor: Accessor<T>, opts: ColOpts<T> = {}): TableColumn<T> {
  return {
    align: 'right',
    sortType: 'number',
    ...opts,
    className: joinClass(NUMBER_CLASS, opts.className),
    id,
    header,
    accessor,
  };
}

/**
 * 完全自定义的列。不传 accessor 时值恒为空，且默认不可排序；
 * 传了 accessor 则与 textCol 的默认一致。
 */
export function customCol<T>(
  id: string,
  header: string,
  opts: ColOpts<T> & { accessor?: Accessor<T> } = {},
): TableColumn<T> {
  const { accessor, ...rest } = opts;
  return {
    sortable: accessor ? undefined : false,
    ...rest,
    id,
    header,
    accessor: accessor ?? (() => null),
  };
}

/** 按 ids 的顺序挑选列；不存在的 id 忽略，重复的 id 只取一次。 */
export function pickColumns<T>(all: TableColumn<T>[], ids: string[]): TableColumn<T>[] {
  const byId = new Map(all.map((c) => [c.id, c]));
  const seen = new Set<string>();
  const out: TableColumn<T>[] = [];
  for (const id of ids) {
    if (seen.has(id)) continue;
    seen.add(id);
    const col = byId.get(id);
    if (col) out.push(col);
  }
  return out;
}
