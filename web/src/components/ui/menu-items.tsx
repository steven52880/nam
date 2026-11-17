import type { ReactNode } from 'react';
import {
  ContextMenuCheckboxItem,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
} from '@/components/ui/context-menu';
import {
  DropdownMenuCheckboxItem,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

export interface MenuItem {
  key: string;
  label: ReactNode;
  onSelect?: () => void;
  disabled?: boolean;
  /** 悬停提示，常用于说明为什么禁用。 */
  title?: string;
  destructive?: boolean;
  /** 勾选项（列显示菜单）；勾选项点击后菜单不关闭。 */
  checked?: boolean;
  /** 子菜单。 */
  items?: MenuItem[];
  separatorBefore?: boolean;
}

export interface MenuItemsProps {
  items: MenuItem[];
  kind: 'context' | 'dropdown';
  /** 菜单顶部的灰色小标题（如渠道行右键菜单顶部的渠道名）。 */
  header?: ReactNode;
}

/** 禁用项只变灰、不可选；不加 pointer-events-none，悬停时仍能看到 title。 */
const DISABLED = 'data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50';

/** 只渲染菜单项；Root 和 Content 由使用方放。 */
export function MenuItems({ items, kind, header }: MenuItemsProps) {
  const Label = kind === 'context' ? ContextMenuLabel : DropdownMenuLabel;
  return (
    <>
      {header != null && header !== false && (
        <Label className="-mx-1 mb-1 max-w-[16rem] truncate border-b px-3 pb-1.5 pt-1 text-xs font-medium">{header}</Label>
      )}
      {items.map((item) => (
        <MenuEntry key={item.key} item={item} kind={kind} />
      ))}
    </>
  );
}

function MenuEntry({ item, kind }: { item: MenuItem; kind: 'context' | 'dropdown' }) {
  const isContext = kind === 'context';
  const Separator = isContext ? ContextMenuSeparator : DropdownMenuSeparator;
  const Item = isContext ? ContextMenuItem : DropdownMenuItem;
  const CheckboxItem = isContext ? ContextMenuCheckboxItem : DropdownMenuCheckboxItem;
  const Sub = isContext ? ContextMenuSub : DropdownMenuSub;
  const SubTrigger = isContext ? ContextMenuSubTrigger : DropdownMenuSubTrigger;
  const SubContent = isContext ? ContextMenuSubContent : DropdownMenuSubContent;

  const className = cn(
    DISABLED,
    item.destructive && 'text-destructive data-[highlighted]:bg-destructive/15 data-[highlighted]:text-destructive',
  );
  // 勾选项：对勾用主题色；两种菜单共用，不用改各自的组件
  const checkClassName = cn(className, '[&>span:first-child]:text-primary data-[state=checked]:font-medium');
  let body: ReactNode;
  if (item.items) {
    body = (
      <Sub>
        <SubTrigger disabled={item.disabled} title={item.title} className={className}>
          {item.label}
        </SubTrigger>
        <SubContent>
          {item.items.map((sub) => (
            <MenuEntry key={sub.key} item={sub} kind={kind} />
          ))}
        </SubContent>
      </Sub>
    );
  } else if (item.checked !== undefined) {
    body = (
      <CheckboxItem
        checked={item.checked}
        disabled={item.disabled}
        title={item.title}
        className={checkClassName}
        onSelect={(e) => e.preventDefault()}
        onCheckedChange={() => item.onSelect?.()}
      >
        {item.label}
      </CheckboxItem>
    );
  } else {
    body = (
      <Item disabled={item.disabled} title={item.title} className={className} onSelect={() => item.onSelect?.()}>
        {item.label}
      </Item>
    );
  }
  return (
    <>
      {item.separatorBefore && <Separator />}
      {body}
    </>
  );
}
