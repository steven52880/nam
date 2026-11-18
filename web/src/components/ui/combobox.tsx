import * as PopoverPrimitive from '@radix-ui/react-popover';
import { Command, useCommandState } from 'cmdk';
import { Check, ChevronDown, Search, X } from 'lucide-react';
import {
  memo,
  useCallback,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from 'react';
import { Popover, PopoverContent } from './popover';
import { cn } from '@/lib/utils';

export interface ComboboxItem {
  value: string; // 稳定的值（如 id 的字符串）
  label: string;
  keywords?: string[]; // 额外的搜索词
  group?: string; // 分组标题
  disabled?: boolean;
}

export interface ComboboxColumn<I> {
  id: string;
  header: string;
  width: number; // rem，右侧数值列固定宽度，保证上下对齐
  render: (item: I) => ReactNode;
}

export interface ComboboxProps<I extends ComboboxItem> {
  items: I[];
  value: string | null;
  onChange: (value: string | null, item: I | null) => void;
  columns?: ComboboxColumn<I>[];
  renderLabel?: (item: I) => ReactNode;
  variant?: 'field' | 'cell' | 'toolbar';
  placeholder?: string;
  emptyText?: ReactNode;
  loading?: boolean;
  searchable?: boolean; // 默认 true；FilterSelect 选项少时传 false
  allowCustom?: boolean; // 列表里没有时显示"使用「xxx」"
  clearable?: boolean; // 显示清除按钮，清除时 onChange(null, null)
  disabled?: boolean;
  title?: string;
  footer?: ReactNode; // 列表底部，例如"同步分组"按钮
  /** 当前值不在 items 里时，触发器显示它；不传则显示值本身。 */
  valueLabel?: ReactNode;
  /** 搜索框下、列表上方的一行小字提示。 */
  notice?: ReactNode;
  /** 覆盖触发器的宽度等。 */
  className?: string;
  /** 覆盖弹窗内容的样式。 */
  contentClassName?: string;
}

const CUSTOM_VALUE = '__combobox_custom__';

/** 只匹配 keywords（第一项是名称），不匹配 value；不区分大小写；前缀命中分数更高。 */
function matchScore(_value: string, search: string, keywords?: string[]): number {
  const q = search.trim().toLowerCase();
  if (!q) return 1;
  if (!keywords) return 0;
  let best = 0;
  for (let i = 0; i < keywords.length; i += 1) {
    const at = keywords[i].toLowerCase().indexOf(q);
    if (at < 0) continue;
    const weight = i === 0 ? 1 : 0.8; // 名称命中优先于别名
    best = Math.max(best, (at === 0 ? 1 : 0.5) * weight);
  }
  return best;
}

const VARIANT = {
  field: {
    box: 'h-7 w-full rounded border border-input bg-background',
    button: 'px-2',
    clear: 'right-6',
    padClear: 'pr-11',
  },
  cell: {
    box: 'h-6 w-full rounded border border-transparent bg-transparent hover:bg-accent/50',
    button: 'px-1',
    clear: 'right-5',
    padClear: 'pr-10',
  },
  toolbar: {
    box: 'h-7 w-28 shrink-0 rounded border border-input bg-background',
    button: 'px-2',
    clear: 'right-6',
    padClear: 'pr-11',
  },
} as const;

function stopKeys(e: KeyboardEvent<HTMLElement>) {
  e.stopPropagation();
}

interface ItemRowProps<I extends ComboboxItem> {
  item: I;
  selected: boolean;
  columns?: ComboboxColumn<I>[];
  renderLabel?: (item: I) => ReactNode;
  onPick: (item: I) => void;
}

function ItemRowImpl<I extends ComboboxItem>({ item, selected, columns, renderLabel, onPick }: ItemRowProps<I>) {
  const keywords = useMemo(() => [item.label, ...(item.keywords ?? [])], [item.label, item.keywords]);
  return (
    <Command.Item
      value={item.value}
      keywords={keywords}
      disabled={item.disabled}
      data-checked={selected ? 'true' : undefined}
      onSelect={() => onPick(item)}
      className="relative flex min-h-7 cursor-default select-none flex-wrap items-center gap-x-2 rounded py-1 pl-6 pr-2 text-base outline-none data-[disabled=true]:pointer-events-none data-[disabled=true]:opacity-50 data-[selected=true]:bg-accent"
    >
      <Check
        className={cn('absolute left-1.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2', selected ? 'opacity-100' : 'opacity-0')}
      />
      <span className="min-w-40 flex-[1_1_10rem] truncate">{renderLabel?.(item) ?? item.label}</span>
      {columns && columns.length > 0 ? (
        <span className="ml-auto flex shrink-0">
          {columns.map((c) => (
            <span key={c.id} className="shrink-0 text-right tabular-nums" style={{ width: `${c.width}rem` }}>
              {c.render(item)}
            </span>
          ))}
        </span>
      ) : null}
    </Command.Item>
  );
}

const ItemRow = memo(ItemRowImpl) as typeof ItemRowImpl;

function ColumnHeader<I>({ columns }: { columns: ComboboxColumn<I>[] }) {
  const count = useCommandState((s) => s.filtered.count);
  if (count === 0) return null;
  return (
    <div className="sticky top-0 z-10 flex justify-end border-b bg-popover py-0.5 pr-3 text-xs text-muted-foreground">
      {columns.map((c) => (
        <span key={c.id} className="shrink-0 text-right" style={{ width: `${c.width}rem` }}>
          {c.header}
        </span>
      ))}
    </div>
  );
}

const GROUP_CLASS =
  'overflow-hidden text-foreground [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:text-muted-foreground';

export function Combobox<I extends ComboboxItem>({
  items,
  value,
  onChange,
  columns,
  renderLabel,
  variant = 'field',
  placeholder = '请选择',
  emptyText,
  loading,
  searchable = true,
  allowCustom,
  clearable,
  disabled,
  title,
  footer,
  valueLabel,
  notice,
  className,
  contentClassName,
}: ComboboxProps<I>) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [active, setActive] = useState('');
  const triggerRef = useRef<HTMLButtonElement>(null);
  const commandRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const v = VARIANT[variant];
  const isCell = variant === 'cell';

  const selectedItem = useMemo(
    () => (value === null ? undefined : items.find((i) => i.value === value)),
    [items, value],
  );
  const hasValue = value !== null && (selectedItem !== undefined || value !== '');
  const showClear = !!clearable && hasValue && !disabled;

  // 按 group 首次出现的顺序分组；没有 group 的放在同一个无标题组里
  const sections = useMemo(() => {
    const map = new Map<string, I[]>();
    for (const it of items) {
      const key = it.group ?? '';
      const list = map.get(key);
      if (list) list.push(it);
      else map.set(key, [it]);
    }
    return Array.from(map, ([key, list]) => ({ key, list }));
  }, [items]);

  const customText = search.trim();
  const showCustom = useMemo(
    () => !!allowCustom && customText !== '' && !items.some((i) => i.label === customText || i.value === customText),
    [allowCustom, customText, items],
  );

  const handleOpenChange = useCallback(
    (next: boolean) => {
      if (next) {
        const first = items.find((i) => !i.disabled);
        setActive(selectedItem?.value.trim() ?? first?.value.trim() ?? '');
      } else {
        setSearch('');
      }
      setOpen(next);
    },
    [items, selectedItem],
  );

  const pick = useCallback(
    (item: I) => {
      onChangeRef.current(item.value, item);
      handleOpenChange(false);
    },
    [handleOpenChange],
  );

  const pickCustom = () => {
    onChangeRef.current(customText, null);
    handleOpenChange(false);
  };

  const onTriggerKeyDown = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === 'ArrowDown' && !open) {
      e.preventDefault();
      handleOpenChange(true);
    }
  };

  const hasColumns = !!columns && columns.length > 0;

  return (
    <Popover open={open} onOpenChange={handleOpenChange} modal>
      <div
        className={cn(
          'relative inline-flex items-center text-base focus-within:ring-1 focus-within:ring-ring',
          v.box,
          disabled && 'cursor-not-allowed opacity-50',
          className,
        )}
        data-no-row-click={isCell ? '' : undefined}
        onKeyDown={isCell ? stopKeys : undefined}
      >
        <PopoverPrimitive.Trigger asChild>
          <button
            ref={triggerRef}
            type="button"
            role="combobox"
            aria-expanded={open}
            aria-haspopup="listbox"
            disabled={disabled}
            title={title}
            data-no-row-click=""
            onKeyDown={onTriggerKeyDown}
            className={cn(
              'flex h-full w-full min-w-0 items-center justify-between gap-1 rounded text-left outline-none disabled:cursor-not-allowed',
              v.button,
              showClear && v.padClear,
            )}
          >
            <span className={cn('min-w-0 flex-1 truncate', !hasValue && 'text-muted-foreground')}>
              {hasValue ? (selectedItem ? (renderLabel?.(selectedItem) ?? selectedItem.label) : (valueLabel ?? value)) : placeholder}
            </span>
            <ChevronDown className="h-3.5 w-3.5 shrink-0 opacity-60" />
          </button>
        </PopoverPrimitive.Trigger>
        {showClear ? (
          <button
            type="button"
            aria-label="清除"
            data-no-row-click=""
            onClick={(e) => {
              e.stopPropagation();
              onChange(null, null);
              // 清除按钮会随之消失，把焦点还给触发器，键盘用户不会丢失位置
              triggerRef.current?.focus({ preventScroll: true });
            }}
            className={cn(
              'absolute top-1/2 -translate-y-1/2 rounded p-0.5 opacity-60 hover:bg-accent hover:opacity-100',
              v.clear,
            )}
          >
            <X className="h-3.5 w-3.5" />
          </button>
        ) : null}
      </div>
      <PopoverContent
        side="bottom"
        align="start"
        sideOffset={2}
        collisionPadding={8}
        data-no-row-click={isCell ? '' : undefined}
        onKeyDown={isCell ? stopKeys : undefined}
        onOpenAutoFocus={(e) => {
          e.preventDefault();
          (searchable ? inputRef.current : commandRef.current)?.focus({ preventScroll: true });
          const list = listRef.current;
          const checked = list?.querySelector<HTMLElement>('[cmdk-item][data-checked="true"]');
          if (list && checked) list.scrollTop = Math.max(0, checked.offsetTop - list.clientHeight / 2);
        }}
        className={cn(
          'w-max min-w-[var(--radix-popover-trigger-width)] max-w-[min(32rem,var(--radix-popover-content-available-width))] overflow-hidden p-0',
          contentClassName,
        )}
      >
        <Command
          ref={commandRef}
          tabIndex={-1}
          label={title ?? placeholder}
          filter={matchScore}
          loop
          value={active}
          onValueChange={setActive}
          className="flex flex-col outline-none"
        >
          {searchable ? (
            <div className="flex items-center gap-2 border-b px-2">
              <Search className="h-3.5 w-3.5 shrink-0 opacity-60" />
              <Command.Input
                ref={inputRef}
                value={search}
                onValueChange={setSearch}
                placeholder="搜索…"
                className="h-7 w-full bg-transparent text-base outline-none placeholder:text-muted-foreground"
              />
            </div>
          ) : null}
          {notice ? <div className="border-b px-2 py-1 text-xs text-muted-foreground">{notice}</div> : null}
          <Command.List ref={listRef} className="relative max-h-72 overflow-y-auto">
            {hasColumns && !loading ? <ColumnHeader columns={columns} /> : null}
            {loading ? <div className="py-6 text-center text-base text-muted-foreground">加载中…</div> : null}
            {!loading && !showCustom ? (
              <Command.Empty className="py-6 text-center text-base text-muted-foreground">
                {emptyText ?? '无匹配项'}
              </Command.Empty>
            ) : null}
            <div className="p-1">
              {sections.map((s) => (
                <Command.Group key={s.key} heading={s.key || undefined} className={GROUP_CLASS}>
                  {s.list.map((it) => (
                    <ItemRow
                      key={it.value}
                      item={it}
                      selected={it.value === value}
                      columns={columns}
                      renderLabel={renderLabel}
                      onPick={pick}
                    />
                  ))}
                </Command.Group>
              ))}
              {showCustom ? (
                <Command.Group forceMount className={GROUP_CLASS}>
                  <Command.Item
                    forceMount
                    value={CUSTOM_VALUE}
                    onSelect={pickCustom}
                    className="relative flex min-h-7 cursor-default select-none items-center rounded py-1 pl-6 pr-2 text-base outline-none data-[selected=true]:bg-accent"
                  >
                    <span className="truncate">使用「{customText}」</span>
                  </Command.Item>
                </Command.Group>
              ) : null}
            </div>
          </Command.List>
        </Command>
        {footer ? <div className="border-t p-1">{footer}</div> : null}
      </PopoverContent>
    </Popover>
  );
}
