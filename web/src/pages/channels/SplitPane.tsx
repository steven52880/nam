import { useCallback, useRef, useState, type PointerEvent as ReactPointerEvent, type ReactNode } from 'react';

interface SplitPaneProps {
  top: ReactNode;
  bottom: ReactNode;
  /** 下栏初始高度（像素）。 */
  initialBottom?: number;
}

const MIN_PANE = 90;

/** 上下分栏，中间的分隔条可拖动调整下栏高度。 */
export function SplitPane({ top, bottom, initialBottom = 220 }: SplitPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [bottomHeight, setBottomHeight] = useState(initialBottom);

  const onPointerDown = useCallback((e: ReactPointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const container = containerRef.current;
    if (!container) return;
    const onMove = (ev: PointerEvent) => {
      const rect = container.getBoundingClientRect();
      const next = rect.bottom - ev.clientY;
      setBottomHeight(Math.max(MIN_PANE, Math.min(rect.height - MIN_PANE, next)));
    };
    const onUp = () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  }, []);

  return (
    <div ref={containerRef} className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-0 flex-1 flex-col">{top}</div>
      <div
        role="separator"
        aria-orientation="horizontal"
        title="拖动调整高度"
        onPointerDown={onPointerDown}
        className="group flex h-2 shrink-0 cursor-row-resize items-center justify-center"
      >
        <div className="h-0.5 w-12 rounded bg-border group-hover:bg-primary" />
      </div>
      <div style={{ height: bottomHeight }} className="shrink-0">{bottom}</div>
    </div>
  );
}
