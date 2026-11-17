import { Columns3 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { MenuItems, type MenuItem } from '@/components/ui/menu-items';
import type { TableColumn } from './types';

export const LAYOUT_LOADING_HINT = '布局加载中';

export interface ColumnMenuArgs<T> {
  /** 全部数据列，按显示顺序。 */
  columns: TableColumn<T>[];
  visibility: Record<string, boolean>;
  /** 布局已加载；false 时布局相关的项全部禁用。 */
  editable: boolean;
  /** 在表头某一列上右键时传入；工具栏的"列"按钮传 null。 */
  columnId: string | null;
  onToggle: (id: string, visible: boolean) => void;
  onAutoFitColumn: (id: string) => void;
  onAutoFitAll: () => void;
  onReset: () => void;
}

/** 表头右键菜单和工具栏"列"按钮共用的菜单项。 */
export function buildColumnMenuItems<T>(a: ColumnMenuArgs<T>): MenuItem[] {
  const loading = a.editable ? undefined : LAYOUT_LOADING_HINT;
  const toggleable = a.columns.filter((c) => !c.fixed);
  const visibleCount = toggleable.filter((c) => a.visibility[c.id]).length;
  const target = a.columnId ? toggleable.find((c) => c.id === a.columnId) : undefined;
  const items: MenuItem[] = [];

  if (target) {
    const onlyOne = visibleCount <= 1 && a.visibility[target.id];
    items.push({
      key: 'hide',
      label: '隐藏此列',
      disabled: !a.editable || onlyOne,
      title: loading ?? (onlyOne ? '至少保留一列' : undefined),
      onSelect: () => a.onToggle(target.id, false),
    });
  }

  items.push({
    key: 'columns',
    label: '显示列',
    disabled: !a.editable,
    title: loading,
    separatorBefore: items.length > 0,
    items: toggleable.map((c) => {
      const on = !!a.visibility[c.id];
      const last = on && visibleCount <= 1;
      return {
        key: `col:${c.id}`,
        label: c.header,
        checked: on,
        disabled: !a.editable || last,
        title: loading ?? (last ? '至少保留一列' : undefined),
        onSelect: () => a.onToggle(c.id, !on),
      };
    }),
  });

  if (target) {
    items.push({
      key: 'fit-one',
      label: '自动调整此列',
      disabled: !a.editable,
      title: loading,
      separatorBefore: true,
      onSelect: () => a.onAutoFitColumn(target.id),
    });
  }
  items.push({
    key: 'fit-all',
    label: '自动调整全部列',
    disabled: !a.editable,
    title: loading,
    separatorBefore: !target,
    onSelect: a.onAutoFitAll,
  });
  items.push({
    key: 'reset',
    label: '恢复本表默认布局',
    disabled: !a.editable,
    title: loading,
    separatorBefore: true,
    onSelect: a.onReset,
  });
  return items;
}

/** 工具栏右侧固定的"列"按钮。 */
export function ColumnMenuButton({ items }: { items: MenuItem[] }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="data-[state=open]:bg-accent">
          <Columns3 className="h-3.5 w-3.5" />列
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <MenuItems items={items} kind="dropdown" />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
