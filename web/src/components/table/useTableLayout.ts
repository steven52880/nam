import { useCallback, useMemo, useRef } from 'react';
import { useLayoutEditable, useLayoutSlot } from '@/lib/layout/LayoutProvider';
import { tableSlotId, type TableLayoutId } from '@/lib/layout/slots';
import { stableStringify } from '@/lib/layout/stable';
import type { SlotSpec } from '@/lib/layout/store';
import {
  defaultTableLayout,
  diffVisibility,
  diffWidths,
  effectiveVisibility,
  parseTableLayout,
  reconcileOrder,
  reconcileWidths,
} from './layout';
import type { SortSpec, TableColumn, TableLayout } from './types';

const specs = new Map<TableLayoutId, SlotSpec<TableLayout>>();

/** 同一个 layoutId 永远返回同一个 spec 对象（useSyncExternalStore 需要稳定的订阅）。 */
export function tableLayoutSpec(id: TableLayoutId): SlotSpec<TableLayout> {
  let spec = specs.get(id);
  if (!spec) {
    spec = { id: tableSlotId(id), version: 1, defaults: defaultTableLayout, parse: parseTableLayout };
    specs.set(id, spec);
  }
  return spec;
}

function keepAbsent<V>(saved: Record<string, V>, present: ReadonlySet<string>): Record<string, V> {
  const out: Record<string, V> = {};
  for (const [id, v] of Object.entries(saved)) if (!present.has(id)) out[id] = v;
  return out;
}

export interface TableLayoutState<T> {
  /** 布局已加载完成，允许调整。 */
  editable: boolean;
  /** 全部列，按显示顺序（含隐藏列；fixed 列保持在默认位置）。 */
  ordered: TableColumn<T>[];
  /** 非 fixed 列的对账后顺序。 */
  order: string[];
  /** 每列是否可见（fixed 列恒为 true）。 */
  visibility: Record<string, boolean>;
  /** 可见列，按显示顺序。 */
  visible: TableColumn<T>[];
  /** 每列宽度（像素）。 */
  widths: Record<string, number>;
  /** 用户排序（已排除不存在的列）。 */
  userSort: SortSpec | null;
  /** px：每列当前宽度（像素）。ceilIds 里的列（自动调整）向上取到 0.25rem，其它四舍五入。 */
  commitWidths(px: Record<string, number>, opts?: { ceilIds?: ReadonlySet<string> }): void;
  commitOrder(order: string[]): void;
  setVisible(id: string, visible: boolean): void;
  setSort(sort: SortSpec | null): void;
  reset(): void;
}

/**
 * 布局槽位 ↔ 表格状态。对账只在读取时计算，挂载时不写回；
 * 用户操作后才写槽位（写入时保留已保存但当前不存在的列的记录）。
 */
export function useTableLayout<T>(layoutId: TableLayoutId, columns: TableColumn<T>[]): TableLayoutState<T> {
  const [saved, setSaved, resetSlot] = useLayoutSlot(tableLayoutSpec(layoutId));
  const editable = useLayoutEditable();

  const nonFixedIds = useMemo(() => columns.filter((c) => !c.fixed).map((c) => c.id), [columns]);
  const order = useMemo(() => reconcileOrder(saved.order, nonFixedIds, nonFixedIds), [saved.order, nonFixedIds]);

  const ordered = useMemo(() => {
    const byId = new Map(columns.map((c) => [c.id, c]));
    let next = 0;
    return columns.map((c) => {
      if (c.fixed) return c;
      return byId.get(order[next++]) ?? c;
    });
  }, [columns, order]);

  const visibility = useMemo(() => effectiveVisibility(saved.visibility, columns), [saved.visibility, columns]);
  const visible = useMemo(() => ordered.filter((c) => visibility[c.id]), [ordered, visibility]);
  const widths = useMemo(() => reconcileWidths(saved.widths, columns), [saved.widths, columns]);
  const userSort = useMemo(
    () => (saved.sort && columns.some((c) => c.id === saved.sort?.id) ? saved.sort : null),
    [saved.sort, columns],
  );

  const savedRef = useRef(saved);
  savedRef.current = saved;
  const columnsRef = useRef(columns);
  columnsRef.current = columns;
  const visibilityRef = useRef(visibility);
  visibilityRef.current = visibility;
  const editableRef = useRef(editable);
  editableRef.current = editable;

  const write = useCallback(
    (patch: Partial<TableLayout>) => {
      if (!editableRef.current) return;
      const next = { ...savedRef.current, ...patch };
      // 没有实际变化（例如只点了一下改宽手柄）就不写，避免产生"未保存"
      if (stableStringify(next) === stableStringify(savedRef.current)) return;
      savedRef.current = next;
      setSaved(next);
    },
    [setSaved],
  );

  const commitWidths = useCallback(
    (px: Record<string, number>, opts?: { ceilIds?: ReadonlySet<string> }) => {
      const cols = columnsRef.current;
      const present = new Set(cols.map((c) => c.id));
      write({ widths: { ...keepAbsent(savedRef.current.widths, present), ...diffWidths(px, cols, opts) } });
    },
    [write],
  );

  const commitOrder = useCallback(
    (next: string[]) => {
      const present = new Set(columnsRef.current.map((c) => c.id));
      write({ order: [...next, ...savedRef.current.order.filter((id) => !present.has(id))] });
    },
    [write],
  );

  const setVisible = useCallback(
    (id: string, visible: boolean) => {
      const cols = columnsRef.current;
      const present = new Set(cols.map((c) => c.id));
      const next = { ...visibilityRef.current, [id]: visible };
      write({ visibility: { ...keepAbsent(savedRef.current.visibility, present), ...diffVisibility(next, cols) } });
    },
    [write],
  );

  const setSort = useCallback((sort: SortSpec | null) => write({ sort }), [write]);

  const reset = useCallback(() => {
    if (!editableRef.current) return;
    resetSlot();
  }, [resetSlot]);

  return { editable, ordered, order, visibility, visible, widths, userSort, commitWidths, commitOrder, setVisible, setSort, reset };
}
