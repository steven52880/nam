import { useEffect, useRef } from 'react';
import type { ChannelRow } from '@/api/types';
import { Button } from '@/components/ui/button';

export interface MenuItem {
  label: string;
  onSelect: () => void;
  destructive?: boolean;
  disabled?: boolean;
  /** 悬停提示，常用于说明为什么禁用。 */
  title?: string;
}

interface RowContextMenuProps {
  state: { row: ChannelRow; x: number; y: number } | null;
  items: (row: ChannelRow) => MenuItem[];
  onClose: () => void;
}

/** 简单的右键菜单：固定定位在鼠标处，点击外部或按 Esc 关闭。 */
export function RowContextMenu({ state, items, onClose }: RowContextMenuProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!state) return;
    const onDown = (e: PointerEvent) => {
      if (e.target instanceof Node && ref.current?.contains(e.target)) return;
      onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('pointerdown', onDown);
    window.addEventListener('keydown', onKey);
    window.addEventListener('blur', onClose);
    return () => {
      window.removeEventListener('pointerdown', onDown);
      window.removeEventListener('keydown', onKey);
      window.removeEventListener('blur', onClose);
    };
  }, [state, onClose]);

  if (!state) return null;
  const left = Math.min(state.x, window.innerWidth - 180);
  const top = Math.min(state.y, window.innerHeight - 240);
  return (
    <div
      ref={ref}
      style={{ left, top }}
      className="fixed z-[60] min-w-40 rounded-md border bg-popover p-1 shadow-md"
      onContextMenu={(e) => e.preventDefault()}
    >
      <div className="truncate px-2 py-1 text-xs text-muted-foreground">{state.row.name}</div>
      {items(state.row).map((item) => (
        <Button
          key={item.label}
          variant="ghost"
          disabled={item.disabled}
          title={item.title}
          className={`h-7 w-full justify-start font-normal ${item.destructive ? 'text-destructive' : ''}`}
          onClick={() => {
            onClose();
            item.onSelect();
          }}
        >
          {item.label}
        </Button>
      ))}
    </div>
  );
}
