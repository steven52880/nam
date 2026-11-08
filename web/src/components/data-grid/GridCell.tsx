import { useState } from 'react';
import { cn } from '@/lib/utils';
import type { GridColumn } from './types';

interface GridCellProps<T> {
  row: T;
  rowId: string;
  column: GridColumn<T>;
  selected: boolean;
  focused: boolean;
  dirty: boolean;
  className?: string;
  editing: boolean;
  onCommit: (value: string) => void;
  onCancel: () => void;
  onStartEdit: () => void;
}

export function cellText<T>(column: GridColumn<T>, row: T): string {
  if (column.toText) return column.toText(row);
  const v = column.accessor(row);
  return v === null || v === undefined ? '' : String(v);
}

export function GridCell<T>({
  row,
  rowId,
  column,
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
      data-sheet-cell="true"
      data-row-id={rowId}
      data-col-id={column.id}
      onDoubleClick={onStartEdit}
      className={cn(
        'relative h-7 scroll-mt-7 truncate border-b border-r px-2 text-base',
        column.align === 'right' && 'text-right tabular-nums',
        readOnly && 'text-muted-foreground',
        dirty && 'bg-warning/20',
        selected && 'bg-primary/15',
        focused && 'outline outline-2 -outline-offset-2 outline-primary',
        className,
      )}
    >
      {dirty && <span className="absolute left-0 top-0 h-0 w-0 border-l-[6px] border-t-[6px] border-l-warning border-t-transparent" />}
      {editing ? (
        <CellEditor initial={cellText(column, row)} align={column.align} onCommit={onCommit} onCancel={onCancel} />
      ) : column.cell ? (
        column.cell(row)
      ) : (
        cellText(column, row)
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
