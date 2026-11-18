import type { Header } from '@tanstack/react-table';
import { ArrowDown, ArrowUp } from 'lucide-react';
import type { MutableRefObject, ReactNode } from 'react';
import { useSortableItem } from '@/components/dnd/Sortable';
import { cn } from '@/lib/utils';
import type { DisplayCol, SortSpec } from './types';
import { colSortId } from './useTableDnd';

interface HeaderRowProps<T> {
  headers: Header<T, unknown>[];
  displayById: ReadonlyMap<string, DisplayCol<T>>;
  /** 当前生效的排序（用户排序或默认排序），用来显示箭头。 */
  sortSpecs: SortSpec[];
  /** false 时（服务端分页、拖动排序行）表头不可点。 */
  canSort: boolean;
  onSort: (columnId: string) => void;
  /** 布局已加载：false 时隐藏改宽手柄、列不能拖。 */
  editable: boolean;
  /** 刚拖完的标记：同一次操作产生的 click 不触发排序。 */
  justDragged: MutableRefObject<boolean>;
  suppressUntilRelease: () => void;
  onAutoFit: (columnId: string) => void;
  /** 勾选列的表头内容（多选时是三态勾选框）。 */
  selectHeader: ReactNode;
}

/** 表头一行。拖列宽时它会跟着重新渲染，所以数据行不放在这里。 */
export function HeaderRow<T>({ headers, displayById, ...rest }: HeaderRowProps<T>) {
  return (
    <tr>
      {headers.map((h) => {
        const dc = displayById.get(h.column.id);
        if (!dc) return null;
        return <HeaderCell key={h.id} header={h} dc={dc} {...rest} />;
      })}
    </tr>
  );
}

type CellProps<T> = Omit<HeaderRowProps<T>, 'headers' | 'displayById'> & {
  header: Header<T, unknown>;
  dc: DisplayCol<T>;
};

function HeaderCell<T>({
  header,
  dc,
  sortSpecs,
  canSort,
  onSort,
  editable,
  justDragged,
  suppressUntilRelease,
  onAutoFit,
  selectHeader,
}: CellProps<T>) {
  const id = header.column.id;
  const col = dc.kind === 'data' ? dc.col : null;
  const draggable = !!col && !col.fixed && editable;
  const sortable = !!col && canSort && col.sortable !== false;
  const resizable = !!col && !col.fixed && editable;
  const s = useSortableItem(colSortId(id), 'column', { disabled: !draggable });
  const dir = sortSpecs.find((x) => x.id === id);
  const resize = header.getResizeHandler();

  return (
    <th
      ref={s.setNodeRef}
      style={s.style}
      data-col-id={id}
      title={col?.headerTitle ?? (col && !col.fixed && !editable ? '布局未加载，暂时不能调整列' : undefined)}
      {...(draggable ? s.attributes : {})}
      {...(draggable ? s.listeners : {})}
      role="columnheader"
      aria-sort={dir ? (dir.desc ? 'descending' : 'ascending') : undefined}
      onClick={
        sortable
          ? () => {
              if (justDragged.current) return;
              onSort(id);
            }
          : undefined
      }
      className={cn(
        'relative h-7 select-none border-b border-r px-2 text-left text-sm font-medium text-muted-foreground',
        'focus-visible:outline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary',
        col?.align === 'right' && 'text-right',
        dc.kind !== 'data' && 'px-1 text-center',
        sortable && 'cursor-pointer hover:text-foreground',
        dir && 'text-foreground',
        // 可拖动的列用抓手光标；点击仍然是排序
        draggable && 'cursor-grab active:cursor-grabbing',
        // 其它列拖过来时：底部一条主题色线提示插入位置
        s.isOver && !s.isDragging && 'bg-primary/10 shadow-[inset_0_-0.125rem_0_0_hsl(var(--primary))]',
        // 被拖的列：半透明、浮在上层
        s.isDragging && 'z-20 cursor-grabbing bg-accent opacity-60 shadow-md',
      )}
    >
      <div data-header-content className="inline-flex max-w-full items-center gap-1 truncate align-middle">
        {dc.kind === 'select' ? selectHeader : col?.header}
        {dir &&
          (dir.desc ? (
            <ArrowDown aria-hidden className="h-3.5 w-3.5 shrink-0 text-primary" />
          ) : (
            <ArrowUp aria-hidden className="h-3.5 w-3.5 shrink-0 text-primary" />
          ))}
      </div>
      {resizable && (
        <div
          role="separator"
          aria-orientation="vertical"
          data-no-row-click
          onPointerDown={(e) => e.stopPropagation()}
          onMouseDown={(e) => {
            e.stopPropagation();
            suppressUntilRelease();
            resize(e);
          }}
          onTouchStart={(e) => {
            e.stopPropagation();
            suppressUntilRelease();
            resize(e);
          }}
          onClick={(e) => e.stopPropagation()}
          onDoubleClick={(e) => {
            e.stopPropagation();
            onAutoFit(id);
          }}
          className={cn(
            // 热区 0.5rem 宽，完全在本列内；可见的竖线用伪元素画，平时透明
            'group absolute right-0 top-0 z-10 h-full w-2 cursor-col-resize touch-none select-none',
            'after:absolute after:inset-y-1 after:right-0 after:w-0.5 after:rounded-full after:bg-transparent after:transition-colors',
            'hover:after:bg-primary/60',
            header.column.getIsResizing() && 'after:inset-y-0 after:bg-primary hover:after:bg-primary',
          )}
        />
      )}
    </th>
  );
}
