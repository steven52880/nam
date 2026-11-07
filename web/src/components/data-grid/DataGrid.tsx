import {
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type VisibilityState,
} from '@tanstack/react-table';
import { ArrowDown, ArrowUp, ChevronDown, ChevronRight } from 'lucide-react';
import { useCallback, useMemo, useRef, useState, type ClipboardEvent, type KeyboardEvent, type ReactNode } from 'react';
import { toast } from 'sonner';
import { useGridSelection } from '@/lib/selection/useGridSelection';
import { cn } from '@/lib/utils';
import { ColumnVisibilityMenu, GridHintBar } from './GridChrome';
import { GridCell, cellText } from './GridCell';
import type { GridColumn, GridRange } from './types';

export interface DataGridProps<T> {
  data: T[];
  columns: GridColumn<T>[];
  getRowId: (row: T) => string;
  /** 分组键；提供后按键分组并显示可折叠的组标题行。 */
  groupBy?: (row: T) => string;
  groupSummary?: (groupKey: string, rows: T[]) => ReactNode;
  isDirty?: (rowId: string, columnId: string) => boolean;
  /** 单元格提交（已通过校验）。 */
  onEdit?: (rowId: string, columnId: string, value: string) => void;
  /** Ctrl+C。返回文本时写入系统剪贴板。 */
  onCopy?: (range: GridRange) => string | void;
  /** Ctrl+V。text 为剪贴板文本。 */
  onPaste?: (range: GridRange, text: string) => void;
  toolbar?: ReactNode;
  className?: string;
}

type Item<T> = { kind: 'group'; key: string; rows: T[] } | { kind: 'row'; id: string; row: T };

export function DataGrid<T>({
  data,
  columns,
  getRowId,
  groupBy,
  groupSummary,
  isDirty,
  onEdit,
  onCopy,
  onPaste,
  toolbar,
  className,
}: DataGridProps<T>) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [sorting, setSorting] = useState<SortingState>([]);
  const [visibility, setVisibility] = useState<VisibilityState>(() =>
    Object.fromEntries(columns.filter((c) => c.hidden).map((c) => [c.id, false])),
  );
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<{ rowId: string; columnId: string } | null>(null);

  const colMap = useMemo(() => new Map(columns.map((c) => [c.id, c])), [columns]);

  const defs = useMemo<ColumnDef<T>[]>(
    () =>
      columns.map((c) => ({
        id: c.id,
        header: c.header,
        accessorFn: (row: T) => c.accessor(row) ?? '',
        enableSorting: c.sortable !== false,
      })),
    [columns],
  );

  const table = useReactTable({
    data,
    columns: defs,
    getRowId,
    state: { sorting, columnVisibility: visibility },
    onSortingChange: setSorting,
    onColumnVisibilityChange: setVisibility,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  const sortedRows = table.getRowModel().rows;
  const visibleCols = table.getVisibleLeafColumns();
  const columnIds = useMemo(() => visibleCols.map((c) => c.id), [visibleCols]);

  const items = useMemo<Item<T>[]>(() => {
    if (!groupBy) return sortedRows.map((r) => ({ kind: 'row', id: r.id, row: r.original }));
    const groups = new Map<string, T[]>();
    const order: string[] = [];
    for (const r of sortedRows) {
      const k = groupBy(r.original);
      if (!groups.has(k)) {
        groups.set(k, []);
        order.push(k);
      }
      groups.get(k)?.push(r.original);
    }
    const out: Item<T>[] = [];
    for (const k of order) {
      const rows = groups.get(k) ?? [];
      out.push({ kind: 'group', key: k, rows });
      if (!collapsed.has(k)) for (const row of rows) out.push({ kind: 'row', id: getRowId(row), row });
    }
    return out;
  }, [sortedRows, groupBy, collapsed, getRowId]);

  const rowIds = useMemo(
    () => items.flatMap((i) => (i.kind === 'row' ? [i.id] : [])),
    [items],
  );

  const sel = useGridSelection({ containerRef, rowIds, columnIds });
  const rowById = useMemo(() => new Map(data.map((r) => [getRowId(r), r])), [data, getRowId]);

  const startEdit = useCallback(
    (rowId: string, columnId: string) => {
      const col = colMap.get(columnId);
      if (!col) return;
      if (!col.editable) {
        toast.info(col.readOnlyReason ?? `「${col.header}」为只读列，不能编辑`);
        return;
      }
      setEditing({ rowId, columnId });
    },
    [colMap],
  );

  const commit = useCallback(
    (rowId: string, columnId: string, value: string) => {
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
    },
    [colMap, rowById, onEdit],
  );

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (editing) return;
    if (sel.handleKey(e)) return;
    const f = sel.focus;
    if (f && (e.key === 'Enter' || e.key === 'F2')) {
      e.preventDefault();
      startEdit(f.rowId, f.columnId);
    }
  };

  const handleCopy = (e: ClipboardEvent<HTMLDivElement>) => {
    if (editing || !sel.rect || !onCopy) return;
    const text = onCopy(sel.rect);
    if (typeof text === 'string') {
      e.clipboardData.setData('text/plain', text);
      e.preventDefault();
    }
  };

  const handlePaste = (e: ClipboardEvent<HTMLDivElement>) => {
    if (editing || !sel.rect || !onPaste) return;
    e.preventDefault();
    onPaste(sel.rect, e.clipboardData.getData('text/plain'));
  };

  const toggleGroup = (key: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  return (
    <div className={cn('flex min-h-0 flex-col rounded border bg-card', className)}>
      <div className="flex h-9 shrink-0 items-center justify-between gap-2 border-b px-2">
        <div className="flex items-center gap-2">{toolbar}</div>
        <ColumnVisibilityMenu table={table} />
      </div>
      <div
        ref={containerRef}
        tabIndex={0}
        onKeyDown={onKeyDown}
        onCopy={handleCopy}
        onPaste={handlePaste}
        onPointerDown={sel.onPointerDown}
        className="min-h-0 flex-1 select-none overflow-auto outline-none"
      >
        <table className="w-max min-w-full border-separate border-spacing-0 text-base">
          <thead className="sticky top-0 z-10 bg-muted">
            <tr>
              {visibleCols.map((vc) => {
                const col = colMap.get(vc.id);
                const dir = vc.getIsSorted();
                return (
                  <th
                    key={vc.id}
                    style={{ minWidth: col?.width ?? 100 }}
                    onClick={vc.getCanSort() ? vc.getToggleSortingHandler() : undefined}
                    className={cn(
                      'h-7 border-b border-r px-2 text-left text-sm font-medium text-muted-foreground',
                      col?.align === 'right' && 'text-right',
                      vc.getCanSort() && 'cursor-pointer hover:text-foreground',
                    )}
                  >
                    <span className="inline-flex items-center gap-1">
                      {col?.header}
                      {dir === 'asc' && <ArrowUp className="h-3 w-3" />}
                      {dir === 'desc' && <ArrowDown className="h-3 w-3" />}
                    </span>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {items.map((item) =>
              item.kind === 'group' ? (
                <tr
                  key={`g:${item.key}`}
                  onClick={() => toggleGroup(item.key)}
                  className="cursor-pointer bg-muted/60 hover:bg-muted"
                >
                  <td colSpan={columnIds.length} className="h-7 border-b px-2 text-sm font-medium">
                    <span className="inline-flex items-center gap-1">
                      {collapsed.has(item.key) ? <ChevronRight className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
                      {item.key || '（空）'}
                      <span className="font-normal text-muted-foreground">· {item.rows.length} 行</span>
                      {groupSummary && <span className="ml-2 font-normal text-muted-foreground">{groupSummary(item.key, item.rows)}</span>}
                    </span>
                  </td>
                </tr>
              ) : (
                <tr key={item.id} className="hover:bg-accent/40">
                  {visibleCols.map((vc) => {
                    const col = colMap.get(vc.id);
                    if (!col) return null;
                    const isEditing = editing?.rowId === item.id && editing.columnId === vc.id;
                    return (
                      <GridCell
                        key={vc.id}
                        row={item.row}
                        rowId={item.id}
                        column={col}
                        selected={sel.isSelected(item.id, vc.id)}
                        focused={sel.isFocus(item.id, vc.id)}
                        dirty={isDirty?.(item.id, vc.id) ?? false}
                        editing={isEditing}
                        onStartEdit={() => startEdit(item.id, vc.id)}
                        onCommit={(v) => commit(item.id, vc.id, v)}
                        onCancel={() => {
                          setEditing(null);
                          containerRef.current?.focus({ preventScroll: true });
                        }}
                      />
                    );
                  })}
                </tr>
              ),
            )}
            {items.length === 0 && (
              <tr>
                <td colSpan={Math.max(1, columnIds.length)} className="h-16 text-center text-muted-foreground">
                  暂无数据
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <GridHintBar range={sel.rect} />
    </div>
  );
}
