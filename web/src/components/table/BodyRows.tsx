import { ChevronDown, ChevronRight, GripVertical } from 'lucide-react';
import { memo, type CSSProperties, type MouseEvent, type ReactNode } from 'react';
import { useSortableItem } from '@/components/dnd/Sortable';
import { cn } from '@/lib/utils';
import { GridCell } from './GridCell';
import type { DisplayCol, TableItem } from './types';
import { groupSortId, rowSortId } from './useTableDnd';

type Sortable = ReturnType<typeof useSortableItem>;

/** 一行拖动相关的 DOM 属性；不可拖动的行为空对象。 */
interface DragProps {
  rowRef?: (el: HTMLElement | null) => void;
  style?: CSSProperties;
  dragging?: boolean;
  /** 整行都是拖动手柄（组标题行）时放在 tr 上。 */
  rowHandle?: object;
  /** 拖动手柄按钮（数据行的 __drag 列）。 */
  handleRef?: (el: HTMLElement | null) => void;
  handle?: object;
}

function dragPropsOf(s: Sortable, whole: boolean): DragProps {
  const handle = { ...s.attributes, ...s.listeners };
  return whole
    ? { rowRef: s.setNodeRef, style: s.style, dragging: s.isDragging, rowHandle: handle }
    : { rowRef: s.setNodeRef, style: s.style, dragging: s.isDragging, handleRef: s.setActivatorNodeRef, handle };
}

export interface BodyRowsProps<T> {
  items: TableItem<T>[];
  /** 当前可见的列，按显示顺序（不含宽度，拖列宽时这里不变）。 */
  cols: DisplayCol<T>[];
  getRowId: (row: T) => string;
  /** selection = 'cells'：单元格带框选属性、可双击编辑。 */
  sheet: boolean;
  isSelected: (rowId: string, columnId: string) => boolean;
  isFocus: (rowId: string, columnId: string) => boolean;
  isDirty?: (rowId: string, columnId: string) => boolean;
  rowClassName?: (row: T) => string | undefined;
  cellClassName?: (row: T, columnId: string) => string | undefined;
  editing: { rowId: string; columnId: string } | null;
  onStartEdit: (rowId: string, columnId: string) => void;
  onCommit: (rowId: string, columnId: string, value: string) => void;
  onCancelEdit: () => void;
  // 分组
  groupLabel?: (key: string) => ReactNode;
  groupSummary?: (key: string, rows: T[]) => ReactNode;
  onToggleGroup: (key: string) => void;
  canDragGroups: boolean;
  // 行
  rowSelection?: { mode: 'single' | 'multi'; selected: ReadonlySet<string>; isDisabled?: (row: T) => boolean };
  onToggleSelected: (row: T) => void;
  onRowActivate: (row: T, event: MouseEvent<HTMLElement>) => void;
  onRowDoubleClick: (row: T, event: MouseEvent<HTMLElement>) => void;
  hasRowClick: boolean;
  canDragRows: boolean;
  renderExpanded?: (row: T) => ReactNode;
}

function BodyRowsInner<T>(p: BodyRowsProps<T>) {
  return (
    <>
      {p.items.map((item) => {
        if (item.kind === 'group') {
          return p.canDragGroups ? (
            <SortableGroupRow key={`g:${item.key}`} item={item} {...p} />
          ) : (
            <GroupRow key={`g:${item.key}`} item={item} drag={{}} {...p} />
          );
        }
        if (item.kind === 'detail') {
          return (
            <tr key={`d:${item.id}`} className="bg-muted/30">
              <td colSpan={Math.max(1, p.cols.length)} className="border-b px-3 py-2 text-base">
                {p.renderExpanded?.(item.row)}
              </td>
            </tr>
          );
        }
        return p.canDragRows ? (
          <SortableDataRow key={`r:${item.id}`} item={item} {...p} />
        ) : (
          <DataRow key={`r:${item.id}`} item={item} drag={{}} {...p} />
        );
      })}
    </>
  );
}

/** 数据行、组标题行、展开行。memo：props 里没有列宽，拖列宽时不会重新渲染。 */
export const BodyRows = memo(BodyRowsInner) as typeof BodyRowsInner;

// ───────── 组标题行 ─────────

type GroupItem<T> = Extract<TableItem<T>, { kind: 'group' }>;

function SortableGroupRow<T>(p: BodyRowsProps<T> & { item: GroupItem<T> }) {
  const s = useSortableItem(groupSortId(p.item.key), 'group');
  return <GroupRow {...p} drag={dragPropsOf(s, true)} />;
}

function GroupRow<T>({
  item,
  cols,
  groupLabel,
  groupSummary,
  onToggleGroup,
  drag,
}: BodyRowsProps<T> & { item: GroupItem<T>; drag: DragProps }) {
  const draggable = drag.rowHandle !== undefined;
  return (
    <tr
      ref={drag.rowRef}
      style={drag.style}
      data-group-key={item.key}
      aria-expanded={item.expanded}
      onClick={() => onToggleGroup(item.key)}
      {...drag.rowHandle}
      className={cn(
        'group/grp cursor-pointer bg-muted/60 hover:bg-muted',
        'focus-visible:outline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary',
        // 可拖动：抓手光标；拖动中：半透明并浮起
        draggable && 'cursor-grab active:cursor-grabbing',
        drag.dragging && 'relative z-10 cursor-grabbing bg-accent opacity-60 shadow-md',
      )}
    >
      <td colSpan={Math.max(1, cols.length)} className="h-7 border-b px-2 text-sm font-medium">
        <span className="flex items-center gap-1.5">
          {draggable && (
            <GripVertical
              aria-hidden
              className="-ml-1 h-3.5 w-3.5 shrink-0 text-muted-foreground/40 transition-colors group-hover/grp:text-muted-foreground"
            />
          )}
          {item.expanded ? (
            <ChevronDown aria-hidden className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          ) : (
            <ChevronRight aria-hidden className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          )}
          <span className="truncate">{groupLabel ? groupLabel(item.key) : item.key || '（空）'}</span>
          <span className="shrink-0 font-normal tabular-nums text-muted-foreground">· {item.rows.length} 行</span>
          {groupSummary && (
            <span className="ml-2 min-w-0 truncate border-l pl-3 font-normal text-muted-foreground">
              {groupSummary(item.key, item.rows)}
            </span>
          )}
        </span>
      </td>
    </tr>
  );
}

// ───────── 数据行 ─────────

type RowItem<T> = Extract<TableItem<T>, { kind: 'row' }>;

function SortableDataRow<T>(p: BodyRowsProps<T> & { item: RowItem<T> }) {
  const s = useSortableItem(rowSortId(p.item.id), 'row');
  return <DataRow {...p} drag={dragPropsOf(s, false)} />;
}

function DataRow<T>({
  item,
  cols,
  sheet,
  isSelected,
  isFocus,
  isDirty,
  rowClassName,
  cellClassName,
  editing,
  onStartEdit,
  onCommit,
  onCancelEdit,
  rowSelection,
  onToggleSelected,
  onRowActivate,
  onRowDoubleClick,
  hasRowClick,
  drag,
}: BodyRowsProps<T> & { item: RowItem<T>; drag: DragProps }) {
  const { id, row } = item;
  const picked = rowSelection?.selected.has(id) ?? false;
  const disabled = rowSelection?.isDisabled?.(row) ?? false;
  const clickable = hasRowClick || (rowSelection?.mode === 'single' && !disabled);
  return (
    <tr
      ref={drag.rowRef}
      style={drag.style}
      data-row-id={id}
      onClick={(e) => onRowActivate(row, e)}
      onDoubleClick={(e) => onRowDoubleClick(row, e)}
      className={cn(
        'hover:bg-accent/40',
        clickable && 'cursor-pointer',
        picked && 'bg-primary/10',
        drag.dragging && 'relative z-10 bg-accent opacity-60 shadow-md',
        rowClassName?.(row),
      )}
    >
      {cols.map((dc) => {
        if (dc.kind === 'select') {
          return (
            <td key={dc.id} data-col-id={dc.id} className="h-7 border-b border-r px-1 text-center">
              <input
                type="checkbox"
                data-no-row-click
                checked={picked}
                disabled={disabled}
                aria-label="选择此行"
                onChange={() => onToggleSelected(row)}
              />
            </td>
          );
        }
        if (dc.kind === 'drag') {
          return (
            <td key={dc.id} data-col-id={dc.id} className="h-7 border-b border-r px-1 text-center">
              <button
                type="button"
                ref={drag.handleRef}
                data-no-row-click
                aria-label="拖动排序"
                className="inline-flex cursor-grab touch-none items-center rounded text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring active:cursor-grabbing"
                {...drag.handle}
              >
                <GripVertical className="h-3.5 w-3.5" />
              </button>
            </td>
          );
        }
        return (
          <GridCell
            key={dc.id}
            row={row}
            rowId={id}
            column={dc.col}
            sheet={sheet}
            selected={sheet && isSelected(id, dc.id)}
            focused={sheet && isFocus(id, dc.id)}
            dirty={isDirty?.(id, dc.id) ?? false}
            className={cellClassName?.(row, dc.id)}
            editing={editing?.rowId === id && editing.columnId === dc.id}
            onStartEdit={onStartEdit}
            onCommit={onCommit}
            onCancel={onCancelEdit}
          />
        );
      })}
    </tr>
  );
}
