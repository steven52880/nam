import { DndContext, DragOverlay } from '@dnd-kit/core';
import { SortableContext, horizontalListSortingStrategy, verticalListSortingStrategy } from '@dnd-kit/sortable';
import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type ColumnSizingInfoState,
  type ColumnSizingState,
  type Updater,
} from '@tanstack/react-table';
import {
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type ClipboardEvent,
  type KeyboardEvent,
  type MouseEvent,
  type PointerEvent,
} from 'react';
import { GripVertical } from 'lucide-react';
import { createPortal } from 'react-dom';
import { toast } from 'sonner';
import { IndeterminateCheckbox } from '@/components/ui/checkbox';
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from '@/components/ui/context-menu';
import { MenuItems } from '@/components/ui/menu-items';
import { useGridSelection } from '@/lib/selection/useGridSelection';
import { remToPx } from '@/lib/size';
import { cn } from '@/lib/utils';
import { BodyRows } from './BodyRows';
import { ColumnMenuButton, buildColumnMenuItems } from './ColumnMenu';
import { GridHintBar } from './GridHintBar';
import { cellText } from './GridCell';
import { HeaderRow } from './HeaderRow';
import { autoFit } from './autoFit';
import { DRAG_COLUMN_ID, SELECT_COLUMN_ID } from './columns';
import { mergeVisibleOrder } from './groupOrder';
import { buildItems } from './items';
import { DEFAULT_MIN_REM, DEFAULT_WIDTH_REM, MAX_WIDTH_REM } from './layout';
import { nextSort, sortRows } from './sorting';
import type { DataTableHandle, DataTableProps, DisplayCol, SortSpec, TableColumn } from './types';
import { colSortId, groupSortId, rowSortId, sortIdValue, useTableDnd } from './useTableDnd';
import { useEvent } from './useEvent';
import { useTableLayout } from './useTableLayout';

/** 行点击、整行选择、展开都忽略来自这些交互控件的事件。 */
const INTERACTIVE = 'button, a, input, textarea, select, label, [role=combobox], [data-no-row-click]';
/** 自动调整列宽的上限（rem）。 */
const AUTO_FIT_MAX_REM = 40;
/** 合成列宽度（rem）。 */
const SELECT_WIDTH_REM = 2.25;
const DRAG_WIDTH_REM = 2;
const EMPTY_SET: ReadonlySet<string> = new Set();
const EMPTY_SORT: SortSpec[] = [];
const EMPTY_IDS: string[] = [];

const IDLE_SIZING_INFO: ColumnSizingInfoState = {
  startOffset: null,
  startSize: null,
  deltaOffset: null,
  deltaPercentage: null,
  isResizingColumn: false,
  columnSizingStart: [],
};

function resolve<S>(updater: Updater<S>, prev: S): S {
  return typeof updater === 'function' ? (updater as (old: S) => S)(prev) : updater;
}

function minPxOf<T>(col: TableColumn<T>): number {
  return remToPx(col.minWidth ?? DEFAULT_MIN_REM);
}

function clampPx<T>(col: TableColumn<T>, px: number): number {
  const min = minPxOf(col);
  return Math.min(Math.max(px, min), Math.max(min, remToPx(MAX_WIDTH_REM)));
}

export function DataTable<T>(props: DataTableProps<T>) {
  const {
    layoutId,
    data,
    columns,
    getRowId,
    defaultSort,
    manualSorting,
    status,
    toolbar,
    className,
    rowClassName,
    cellClassName,
    tableRef,
    groupBy,
    groupLabel,
    sortGroups,
    groupResetKey,
    groupsDefaultExpanded,
    expandAll,
    onGroupReorder,
    groupSummary,
    selection = 'none',
    onEdit,
    onCopy,
    onPaste,
    isDirty,
    onFocusRowChange,
    onSelectionChange,
    rowSelection,
    onRowClick,
    onRowDoubleClick,
    rowMenu,
    rowMenuHeader,
    renderExpanded,
    onRowReorder,
    rowReorderDisabled,
  } = props;

  const sheet = selection === 'cells';

  // 开发模式：互斥规则
  const hasRowSelection = !!rowSelection;
  const hasExpanded = !!renderExpanded;
  const hasRowReorder = !!onRowReorder;
  const hasGroupBy = !!groupBy;
  useEffect(() => {
    if (!import.meta.env.DEV) return;
    if (sheet && (hasRowSelection || hasExpanded || hasRowReorder)) {
      console.error('DataTable：selection="cells" 不能和 rowSelection、renderExpanded、onRowReorder 同时使用');
    }
    if (hasRowReorder && hasGroupBy) console.error('DataTable：onRowReorder 不能和 groupBy 同时使用');
  }, [sheet, hasRowSelection, hasExpanded, hasRowReorder, hasGroupBy]);

  const containerRef = useRef<HTMLDivElement>(null);
  const layout = useTableLayout(layoutId, columns);
  const { editable } = layout;

  const colMap = useMemo(() => new Map(columns.map((c) => [c.id, c])), [columns]);
  const rowSelectable = !!rowSelection;
  const canSort = !manualSorting && !onRowReorder;

  // ───────── 排序（本组件自己做，不用 TanStack 的排序行模型）─────────
  // 布局加载完成前不能写槽位，点表头排序先记在本地。
  const [localSort, setLocalSort] = useState<{ value: SortSpec | null } | null>(null);
  const userSort = editable ? layout.userSort : localSort ? localSort.value : layout.userSort;
  // 必须稳定：它是 sortedRows → items → rowIds → BodyRows 整条链的依赖
  // defaultSort 常常是调用方每次渲染新建的字面量数组：按内容稳定
  const defaultSortKey = defaultSort && defaultSort.length > 0 ? JSON.stringify(defaultSort) : '';
  const stableDefaultSort = useMemo<SortSpec[]>(
    () => (defaultSortKey === '' ? EMPTY_SORT : (JSON.parse(defaultSortKey) as SortSpec[])),
    [defaultSortKey],
  );
  const effectiveSort = useMemo<SortSpec[]>(
    () => (!canSort ? EMPTY_SORT : userSort ? [userSort] : stableDefaultSort),
    [canSort, userSort, stableDefaultSort],
  );
  const sortedRows = useMemo(
    () => (effectiveSort.length === 0 ? data : sortRows(data, effectiveSort, columns)),
    [data, effectiveSort, columns],
  );
  const sortedRef = useRef(sortedRows);
  sortedRef.current = sortedRows;

  const onSort = useEvent((columnId: string) => {
    const next = nextSort(userSort, columnId);
    if (editable) layout.setSort(next);
    else setLocalSort({ value: next });
  });

  // ───────── 展开状态 ─────────
  // set 的含义随 groupsDefaultExpanded 变：默认折叠时是"被展开的组"，默认展开时是"被收起的组"。
  const [groupState, setGroupState] = useState<{ resetKey: string | undefined; set: ReadonlySet<string> }>({
    resetKey: groupResetKey,
    set: EMPTY_SET,
  });
  // groupResetKey 变化时真正清空（渲染期调整状态）；只在返回值里忽略旧集合的话，切回原 key 时旧展开状态会复活
  if (groupState.resetKey !== groupResetKey) setGroupState({ resetKey: groupResetKey, set: EMPTY_SET });
  const toggledGroups = groupState.resetKey === groupResetKey ? groupState.set : EMPTY_SET;
  const expandedGroups = groupsDefaultExpanded ? EMPTY_SET : toggledGroups;
  const collapsedGroups = groupsDefaultExpanded ? toggledGroups : EMPTY_SET;
  const [expandedRows, setExpandedRows] = useState<ReadonlySet<string>>(EMPTY_SET);

  const onToggleGroup = useEvent((key: string) => {
    // 拖完分组后的那次 click 不切换展开
    if (dnd.justDragged.current) return;
    const next = new Set(toggledGroups);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    setGroupState({ resetKey: groupResetKey, set: next });
  });

  // ───────── 拖动 ─────────
  const dnd = useTableDnd({
    getColumnIds: () => draggableColumnIds,
    onColumnReorder: (visibleIds) => layout.commitOrder(mergeVisibleOrder(layout.order, visibleIds)),
    getGroupKeys: () => groupKeys,
    onGroupReorder: (keys) => onGroupReorder?.(keys),
    getRowIds: () => sortedRef.current.map(getRowId),
    onRowReorder: (ids) => onRowReorder?.(ids),
  });

  // ───────── 条目 ─────────
  const items = useMemo(
    () =>
      buildItems(sortedRows, {
        getRowId,
        groupBy,
        sortGroups,
        expandedGroups,
        defaultExpanded: !!groupsDefaultExpanded,
        collapsedGroups,
        expandAll: !!expandAll,
        collapseAll: dnd.collapseAll,
        expandedRows: renderExpanded ? expandedRows : EMPTY_SET,
      }),
    [sortedRows, getRowId, groupBy, sortGroups, expandedGroups, groupsDefaultExpanded, collapsedGroups, expandAll, dnd.collapseAll, expandedRows, renderExpanded],
  );
  const groupKeys = useMemo(() => items.flatMap((i) => (i.kind === 'group' ? [i.key] : [])), [items]);
  const rowIds = useMemo(() => items.flatMap((i) => (i.kind === 'row' ? [i.id] : [])), [items]);

  // ───────── 列 ─────────
  const displayCols = useMemo<DisplayCol<T>[]>(() => {
    const out: DisplayCol<T>[] = [];
    if (onRowReorder) out.push({ kind: 'drag', id: DRAG_COLUMN_ID });
    if (rowSelectable) out.push({ kind: 'select', id: SELECT_COLUMN_ID });
    for (const col of layout.visible) out.push({ kind: 'data', id: col.id, col });
    return out;
  }, [onRowReorder, rowSelectable, layout.visible]);
  const displayById = useMemo(() => new Map(displayCols.map((d) => [d.id, d])), [displayCols]);
  const dataColumnIds = useMemo(() => layout.visible.map((c) => c.id), [layout.visible]);
  const draggableColumnIds = useMemo(() => layout.visible.filter((c) => !c.fixed).map((c) => c.id), [layout.visible]);

  const defs = useMemo<ColumnDef<T>[]>(() => {
    const out: ColumnDef<T>[] = [];
    if (onRowReorder) {
      const size = remToPx(DRAG_WIDTH_REM);
      out.push({ id: DRAG_COLUMN_ID, header: '', size, minSize: size, maxSize: size, enableResizing: false });
    }
    if (rowSelectable) {
      const size = remToPx(SELECT_WIDTH_REM);
      out.push({ id: SELECT_COLUMN_ID, header: '', size, minSize: size, maxSize: size, enableResizing: false });
    }
    for (const c of columns) {
      out.push({
        id: c.id,
        header: c.header,
        accessorFn: (row: T) => c.accessor(row) ?? '',
        size: remToPx(c.width ?? DEFAULT_WIDTH_REM),
        minSize: minPxOf(c),
        maxSize: remToPx(MAX_WIDTH_REM),
        enableResizing: !c.fixed,
      });
    }
    return out;
  }, [columns, onRowReorder, rowSelectable]);

  const columnOrder = useMemo(
    () => [
      ...(onRowReorder ? [DRAG_COLUMN_ID] : []),
      ...(rowSelectable ? [SELECT_COLUMN_ID] : []),
      ...layout.ordered.map((c) => c.id),
    ],
    [onRowReorder, rowSelectable, layout.ordered],
  );

  // ───────── 列宽：拖动过程中只改组件状态，结束时写一次槽位 ─────────
  const [liveSizing, setLiveSizing] = useState<ColumnSizingState | null>(null);
  const liveRef = useRef<ColumnSizingState | null>(null);
  const [sizingInfo, setSizingInfo] = useState<ColumnSizingInfoState>(IDLE_SIZING_INFO);
  const infoRef = useRef<ColumnSizingInfoState>(IDLE_SIZING_INFO);
  const widthsRef = useRef(layout.widths);
  widthsRef.current = layout.widths;
  const columnsRef = useRef(columns);
  columnsRef.current = columns;

  const currentPx = useCallback((): Record<string, number> => {
    const px: Record<string, number> = {};
    for (const c of columnsRef.current) {
      const raw = liveRef.current?.[c.id] ?? widthsRef.current[c.id];
      if (typeof raw === 'number') px[c.id] = clampPx(c, raw);
    }
    return px;
  }, []);

  const finishResize = useEvent(() => {
    const px = currentPx();
    liveRef.current = null;
    setLiveSizing(null);
    layout.commitWidths(px);
  });

  const table = useReactTable<T>({
    data: sortedRows,
    columns: defs,
    getRowId,
    columnResizeMode: 'onChange',
    enableColumnResizing: true,
    state: {
      columnSizing: liveSizing ?? layout.widths,
      columnSizingInfo: sizingInfo,
      columnOrder,
      columnVisibility: layout.visibility,
    },
    onColumnSizingChange: (updater) => {
      if (!editable) return;
      const next = resolve(updater, liveRef.current ?? widthsRef.current);
      liveRef.current = next;
      setLiveSizing(next);
    },
    onColumnSizingInfoChange: (updater) => {
      const prev = infoRef.current;
      const next = resolve(updater, prev);
      infoRef.current = next;
      setSizingInfo(next);
      if (prev.isResizingColumn && !next.isResizingColumn) finishResize();
    },
    getCoreRowModel: getCoreRowModel(),
  });

  const headers = table.getHeaderGroups()[0]?.headers ?? [];
  const leafCols = table.getVisibleLeafColumns();
  const totalWidth = table.getTotalSize();

  // ───────── 自动调整 ─────────
  const fitOne = useCallback(
    (id: string): number | null => {
      const container = containerRef.current;
      const col = columnsRef.current.find((c) => c.id === id);
      if (!container || !col || col.fixed) return null;
      return autoFit(container, id, minPxOf(col), remToPx(AUTO_FIT_MAX_REM));
    },
    [],
  );
  const autoFitColumn = useEvent((id: string) => {
    if (!editable) return;
    const px = fitOne(id);
    if (px !== null) layout.commitWidths({ ...currentPx(), [id]: px }, { ceilIds: new Set([id]) });
  });
  const autoFitAll = useEvent(() => {
    if (!editable) return;
    const next = currentPx();
    const fitted = new Set<string>();
    for (const c of layout.visible) {
      const px = fitOne(c.id);
      if (px !== null) {
        next[c.id] = px;
        fitted.add(c.id);
      }
    }
    layout.commitWidths(next, { ceilIds: fitted });
  });
  const resetLayout = useEvent(() => {
    setLocalSort(null);
    layout.reset();
  });

  useImperativeHandle<DataTableHandle<T>, DataTableHandle<T>>(
    tableRef as React.Ref<DataTableHandle<T>>,
    () => ({
      getRows: () => sortedRef.current,
      resetLayout,
      autoFitAll,
    }),
    [resetLayout, autoFitAll],
  );

  // ───────── 框选与编辑 ─────────
  const sel = useGridSelection({
    containerRef,
    rowIds: sheet ? rowIds : EMPTY_IDS,
    columnIds: sheet ? dataColumnIds : EMPTY_IDS,
  });
  const rowById = useMemo(() => new Map(data.map((r) => [getRowId(r), r])), [data, getRowId]);
  const [editing, setEditing] = useState<{ rowId: string; columnId: string } | null>(null);

  const startEdit = useEvent((rowId: string, columnId: string) => {
    const col = colMap.get(columnId);
    if (!col) return;
    if (!col.editable) {
      toast.info(col.readOnlyReason ?? `「${col.header}」为只读列，不能编辑`);
      return;
    }
    if (col.openEditor) {
      const row = rowById.get(rowId);
      if (row) col.openEditor(row);
      return;
    }
    setEditing({ rowId, columnId });
  });

  const commit = useEvent((rowId: string, columnId: string, value: string) => {
    const col = colMap.get(columnId);
    const row = rowById.get(rowId);
    if (!col || !row) return;
    const err = col.validate?.(value, row) ?? null;
    if (err) {
      toast.error(err);
      return;
    }
    if (value !== cellText(col, row)) onEdit?.(rowId, columnId, value);
    setEditing(null);
    containerRef.current?.focus({ preventScroll: true });
  });

  const cancelEdit = useEvent(() => {
    setEditing(null);
    containerRef.current?.focus({ preventScroll: true });
  });

  // 编辑中的单元格所在列被隐藏、所在行被折叠或筛掉时取消编辑
  useEffect(() => {
    if (editing && (!dataColumnIds.includes(editing.columnId) || !rowIds.includes(editing.rowId))) setEditing(null);
  }, [editing, dataColumnIds, rowIds]);

  // 框选的起点或终点不再可见时清除选区（rect 为 null 说明有端点不在可见行列里）
  const clearSelection = sel.clear;
  useEffect(() => {
    if (sel.focus && !sel.rect) clearSelection();
  }, [sel.focus, sel.rect, clearSelection]);

  const emitSelection = useEvent((r: typeof sel.rect) => onSelectionChange?.(r));
  const emitFocusRow = useEvent((id: string | null) => onFocusRowChange?.(id));
  // sel.rect 每次重新计算都是新对象；按内容去重，父组件重渲染后列重建也不会再次通知（否则会无限更新）
  const selectedRange = sel.rect;
  const selectionKey = selectedRange ? JSON.stringify([selectedRange.rowIds, selectedRange.columnIds]) : 'null';
  const notifiedSelectionKey = useRef('null');
  useEffect(() => {
    if (!sheet || notifiedSelectionKey.current === selectionKey) return;
    notifiedSelectionKey.current = selectionKey;
    emitSelection(selectedRange);
    // selectedRange 的内容由 selectionKey 体现，不放进依赖
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sheet, selectionKey, emitSelection]);
  const focusRowId = sel.focus?.rowId ?? null;
  useEffect(() => {
    if (sheet) emitFocusRow(focusRowId);
  }, [sheet, focusRowId, emitFocusRow]);

  /** 事件是否来自表格内部 DOM（右键菜单、下拉框是 portal，事件会沿 React 树冒泡上来）。 */
  const fromInside = (target: EventTarget | null): boolean =>
    target instanceof Node && !!containerRef.current?.contains(target);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!sheet || !fromInside(e.target) || editing || dnd.active) return;
    // 来自表头、组标题行的按键（拖动的键盘操作）不移动选区
    if (e.target instanceof Element && e.target.closest('thead, [data-group-key]')) return;
    if (sel.handleKey(e)) return;
    const f = sel.focus;
    if (f && (e.key === 'Enter' || e.key === 'F2')) {
      e.preventDefault();
      startEdit(f.rowId, f.columnId);
    }
  };

  const handleCopy = (e: ClipboardEvent<HTMLDivElement>) => {
    if (!sheet || !fromInside(e.target) || editing || !sel.rect || !onCopy) return;
    const text = onCopy(sel.rect);
    if (typeof text === 'string') {
      e.clipboardData.setData('text/plain', text);
      e.preventDefault();
    }
  };

  const handlePaste = (e: ClipboardEvent<HTMLDivElement>) => {
    if (!sheet || !fromInside(e.target) || editing || !sel.rect || !onPaste) return;
    e.preventDefault();
    onPaste(sel.rect, e.clipboardData.getData('text/plain'));
  };

  // ───────── 行选择、点击、展开 ─────────
  const rowSelectionView = useMemo(
    () =>
      rowSelection
        ? { mode: rowSelection.mode, selected: rowSelection.selected, isDisabled: rowSelection.isDisabled }
        : undefined,
    [rowSelection?.mode, rowSelection?.selected, rowSelection?.isDisabled],
  );

  const onToggleSelected = useEvent((row: T) => {
    if (!rowSelection || rowSelection.isDisabled?.(row)) return;
    const id = getRowId(row);
    const next = new Set(rowSelection.selected);
    if (rowSelection.mode === 'single') {
      const had = next.has(id);
      next.clear();
      if (!had) next.add(id);
    } else if (next.has(id)) next.delete(id);
    else next.add(id);
    rowSelection.onChange(next);
  });

  const onRowActivate = useEvent((row: T, e: MouseEvent<HTMLElement>) => {
    if (dnd.justDragged.current) return;
    if (e.target instanceof Element && e.target.closest(INTERACTIVE)) return;
    const id = getRowId(row);
    if (renderExpanded) {
      setExpandedRows((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      });
    }
    if (rowSelection?.mode === 'single' && !rowSelection.isDisabled?.(row)) {
      rowSelection.onChange(new Set([id]));
    }
    onRowClick?.(row);
  });

  const onRowDblClick = useEvent((row: T, e: MouseEvent<HTMLElement>) => {
    if (e.target instanceof Element && e.target.closest(INTERACTIVE)) return;
    onRowDoubleClick?.(row);
  });

  const selectableRows = useMemo(
    () => (rowSelection ? sortedRows.filter((r) => !rowSelection.isDisabled?.(r)) : []),
    [rowSelection, sortedRows],
  );
  const pickedCount = rowSelection ? selectableRows.filter((r) => rowSelection.selected.has(getRowId(r))).length : 0;
  const allPicked = selectableRows.length > 0 && pickedCount === selectableRows.length;
  const selectHeader =
    rowSelection?.mode === 'multi' ? (
      <IndeterminateCheckbox
        data-no-row-click
        aria-label="全选"
        checked={allPicked}
        indeterminate={pickedCount > 0 && !allPicked}
        disabled={selectableRows.length === 0}
        onChange={() => {
          const next = new Set(rowSelection.selected);
          for (const r of selectableRows) {
            if (allPicked) next.delete(getRowId(r));
            else next.add(getRowId(r));
          }
          rowSelection.onChange(next);
        }}
      />
    ) : null;

  // ───────── 菜单 ─────────
  const [headerMenuCol, setHeaderMenuCol] = useState<string | null>(null);
  const [menuRow, setMenuRow] = useState<T | null>(null);

  const columnMenuItems = (columnId: string | null) =>
    buildColumnMenuItems<T>({
      columns: layout.ordered,
      visibility: layout.visibility,
      editable,
      columnId,
      onToggle: layout.setVisible,
      onAutoFitColumn: autoFitColumn,
      onAutoFitAll: autoFitAll,
      onReset: resetLayout,
    });

  const headerColOf = (target: EventTarget | null): string | null => {
    const th = target instanceof Element ? target.closest('th[data-col-id]') : null;
    return th?.getAttribute('data-col-id') ?? null;
  };

  /** 事件目标所在的数据行；组标题行、展开行、空白处、没有菜单项时返回 null。 */
  const menuRowOf = (target: EventTarget | null): T | null => {
    const tr = target instanceof Element ? target.closest('tr[data-row-id]') : null;
    const id = tr?.getAttribute('data-row-id');
    const row = id != null ? rowById.get(id) : undefined;
    return row && rowMenu && rowMenu(row).length > 0 ? row : null;
  };

  const onHeadContextMenu = (e: MouseEvent<HTMLElement>) => setHeaderMenuCol(headerColOf(e.target));

  const onBodyContextMenu = (e: MouseEvent<HTMLElement>) => {
    const row = menuRowOf(e.target);
    // 没找到行（组标题行）或没有菜单项：阻止默认行为，菜单不打开
    if (!row) e.preventDefault();
    setMenuRow(row);
  };

  // 触摸 / 笔的长按：Radix 用定时器打开菜单，不经过 contextmenu 事件，所以要在 pointerdown 时就确定目标。
  // 目标无效时对这次 pointerdown 调用 preventDefault：Radix 的 Trigger 用 composeEventHandlers 包装，
  // 事件已 defaultPrevented 就不会启动长按定时器（读 @radix-ui/react-context-menu 源码确认）。
  // 选这个做法而不是"打开后再关闭"：不会闪一下空菜单，也不依赖关闭时机。
  const onHeadPointerDown = (e: PointerEvent<HTMLElement>) => {
    if (e.pointerType === 'mouse') return;
    setHeaderMenuCol(headerColOf(e.target));
  };

  const onBodyPointerDown = (e: PointerEvent<HTMLElement>) => {
    if (e.pointerType === 'mouse') return;
    const row = menuRowOf(e.target);
    if (!row) e.preventDefault();
    setMenuRow(row);
  };

  // ───────── 渲染 ─────────
  const canDragGroups = editable && !!onGroupReorder;
  const canDragRows = !!onRowReorder && !rowReorderDisabled;

  const body = (
    <tbody
      onContextMenu={rowMenu ? onBodyContextMenu : undefined}
      onPointerDown={rowMenu ? onBodyPointerDown : undefined}
    >
      {status?.error ? (
        <tr>
          <td colSpan={Math.max(1, displayCols.length)} className="h-16 text-center text-destructive">
            {status.error}
          </td>
        </tr>
      ) : status?.loading && data.length === 0 ? (
        <tr>
          <td colSpan={Math.max(1, displayCols.length)} className="h-16 text-center text-muted-foreground">
            加载中…
          </td>
        </tr>
      ) : items.length === 0 ? (
        <tr>
          <td colSpan={Math.max(1, displayCols.length)} className="h-16 text-center text-muted-foreground">
            {status?.empty ?? '暂无数据'}
          </td>
        </tr>
      ) : (
        <BodyRows
          items={items}
          cols={displayCols}
          getRowId={getRowId}
          sheet={sheet}
          isSelected={sel.isSelected}
          isFocus={sel.isFocus}
          isDirty={isDirty}
          rowClassName={rowClassName}
          cellClassName={cellClassName}
          editing={editing}
          onStartEdit={startEdit}
          onCommit={commit}
          onCancelEdit={cancelEdit}
          groupLabel={groupLabel}
          groupSummary={groupSummary}
          onToggleGroup={onToggleGroup}
          canDragGroups={canDragGroups}
          rowSelection={rowSelectionView}
          onToggleSelected={onToggleSelected}
          onRowActivate={onRowActivate}
          onRowDoubleClick={onRowDblClick}
          hasRowClick={!!onRowClick || !!renderExpanded}
          canDragRows={canDragRows}
          renderExpanded={renderExpanded}
        />
      )}
    </tbody>
  );

  const head = (
    <thead
      className="sticky top-0 z-10 bg-muted"
      onContextMenu={onHeadContextMenu}
      onPointerDown={onHeadPointerDown}
    >
      <HeaderRow
        headers={headers}
        displayById={displayById}
        sortSpecs={canSort ? effectiveSort : EMPTY_SORT}
        canSort={canSort}
        onSort={onSort}
        editable={editable}
        justDragged={dnd.justDragged}
        suppressUntilRelease={dnd.suppressUntilRelease}
        onAutoFit={autoFitColumn}
        selectHeader={selectHeader}
      />
    </thead>
  );

  const columnSortIds = useMemo(() => draggableColumnIds.map(colSortId), [draggableColumnIds]);
  const bodySortIds = useMemo(
    () =>
      groupBy
        ? groupKeys.map(groupSortId)
        : onRowReorder
          ? sortedRows.map((r) => rowSortId(getRowId(r)))
          : [],
    [groupBy, groupKeys, onRowReorder, sortedRows, getRowId],
  );

  const overlay = (() => {
    const active = dnd.active;
    if (!active) return null;
    const value = sortIdValue(active.id);
    let text: React.ReactNode = value;
    if (active.type === 'column') text = colMap.get(value)?.header ?? value;
    else if (active.type === 'group') text = groupLabel ? groupLabel(value) : value || '（空）';
    else {
      const row = rowById.get(value);
      const first = layout.visible[0];
      text = row && first ? cellText(first, row) : value;
    }
    return (
      <div className="flex max-w-[20rem] cursor-grabbing items-center gap-1.5 rounded border border-primary/50 bg-card px-2.5 py-1 text-sm font-medium text-foreground shadow-lg ring-1 ring-primary/20">
        <GripVertical aria-hidden className="h-3.5 w-3.5 shrink-0 text-primary" />
        <span className="truncate">{text}</span>
      </div>
    );
  })();

  const tableEl = (
    <table
      className="table-fixed border-separate border-spacing-0 text-base"
      style={{ width: totalWidth }}
    >
      <colgroup>
        {leafCols.map((c) => (
          <col key={c.id} style={{ width: c.getSize() }} />
        ))}
      </colgroup>
      {/* useSortable 读最近一层 SortableContext：表头一层（列），表体一层（组或行，两者互斥） */}
      <SortableContext items={columnSortIds} strategy={horizontalListSortingStrategy}>
        <ContextMenu>
          <ContextMenuTrigger asChild>{head}</ContextMenuTrigger>
          <ContextMenuContent>
            <MenuItems items={columnMenuItems(headerMenuCol)} kind="context" />
          </ContextMenuContent>
        </ContextMenu>
      </SortableContext>
      <SortableContext items={bodySortIds} strategy={verticalListSortingStrategy}>
        {rowMenu ? (
          <ContextMenu>
            <ContextMenuTrigger asChild>{body}</ContextMenuTrigger>
            <ContextMenuContent>
              {menuRow && <MenuItems items={rowMenu(menuRow)} header={rowMenuHeader?.(menuRow)} kind="context" />}
            </ContextMenuContent>
          </ContextMenu>
        ) : (
          body
        )}
      </SortableContext>
    </table>
  );

  return (
    <DndContext
      sensors={dnd.sensors}
      collisionDetection={dnd.collisionDetection}
      modifiers={dnd.modifiers}
      measuring={dnd.measuring}
      accessibility={{ container: document.body }}
      onDragStart={dnd.onDragStart}
      onDragEnd={dnd.onDragEnd}
      onDragCancel={dnd.onDragCancel}
    >
      <div className={cn('flex min-h-0 flex-col rounded border bg-card', className)}>
        <div className="flex h-9 shrink-0 items-center justify-between gap-2 border-b px-2">
          <div className="flex items-center gap-2">{toolbar}</div>
          <ColumnMenuButton items={columnMenuItems(null)} />
        </div>
        <div
          ref={containerRef}
          tabIndex={sheet ? 0 : undefined}
          onKeyDown={onKeyDown}
          onCopy={handleCopy}
          onPaste={handlePaste}
          onPointerDown={sheet ? sel.onPointerDown : undefined}
          className={cn('relative min-h-0 flex-1 overflow-auto outline-none', sheet && 'select-none')}
        >
          {tableEl}
        </div>
        {sheet && <GridHintBar range={sel.rect} />}
      </div>
      {createPortal(<DragOverlay dropAnimation={null}>{overlay}</DragOverlay>, document.body)}
    </DndContext>
  );
}
