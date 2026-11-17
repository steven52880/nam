import { useState } from 'react';
import { cn } from '@/lib/utils';
import type { TableColumn } from './types';

interface GridCellProps<T> {
  row: T;
  rowId: string;
  column: TableColumn<T>;
  /** 框选模式：带上 data-sheet-cell，支持双击编辑。 */
  sheet: boolean;
  selected: boolean;
  focused: boolean;
  dirty: boolean;
  className?: string;
  editing: boolean;
  onCommit: (rowId: string, columnId: string, value: string) => void;
  onCancel: () => void;
  onStartEdit: (rowId: string, columnId: string) => void;
}

export function cellText<T>(column: TableColumn<T>, row: T): string {
  if (column.toText) return column.toText(row);
  const v = column.accessor(row);
  return v === null || v === undefined ? '' : String(v);
}

export function GridCell<T>({
  row,
  rowId,
  column,
  sheet,
  selected,
  focused,
  dirty,
  className,
  editing,
  onCommit,
  onCancel,
  onStartEdit,
}: GridCellProps<T>) {
  const readOnly = !column.editable;
  return (
    <td
      data-sheet-cell={sheet ? 'true' : undefined}
      data-row-id={rowId}
      data-col-id={column.id}
      onDoubleClick={sheet ? () => onStartEdit(rowId, column.id) : undefined}
      className={cn(
        'relative h-7 scroll-mt-7 overflow-hidden border-b border-r px-2 text-base',
        column.align === 'right' && 'text-right',
        sheet && readOnly && 'text-muted-foreground',
        dirty && 'bg-warning/20',
        selected && 'bg-primary/15',
        focused && 'outline outline-2 -outline-offset-2 outline-primary',
        className,
      )}
    >
      {dirty && (
        <span className="absolute left-0 top-0 h-0 w-0 border-l-[0.375rem] border-t-[0.375rem] border-l-warning border-t-transparent" />
      )}
      <div data-cell-content className={cn('truncate', column.className)}>
        {column.cell ? column.cell(row) : cellText(column, row)}
      </div>
      {editing && (
        <CellEditor
          initial={cellText(column, row)}
          align={column.align}
          onCommit={(v) => onCommit(rowId, column.id, v)}
          onCancel={onCancel}
        />
      )}
    </td>
  );
}

function CellEditor({
  initial,
  align,
  onCommit,
  onCancel,
}: {
  initial: string;
  align?: 'left' | 'right';
  onCommit: (value: string) => void;
  onCancel: () => void;
}) {
  const [value, setValue] = useState(initial);
  return (
    <input
      autoFocus
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onFocus={(e) => e.currentTarget.select()}
      onBlur={onCancel}
      onKeyDown={(e) => {
        e.stopPropagation();
        if (e.key === 'Enter') {
          e.preventDefault();
          onCommit(value);
        } else if (e.key === 'Escape') {
          e.preventDefault();
          onCancel();
        }
      }}
      className={cn(
        'absolute inset-0 h-full w-full bg-background px-2 text-base outline outline-2 -outline-offset-2 outline-primary',
        align === 'right' && 'text-right',
      )}
    />
  );
}
