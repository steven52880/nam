import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from 'react';
import { remToPx } from '@/lib/size';
import { cn } from '@/lib/utils';

export interface SplitPaneProps {
  top: ReactNode;
  bottom: ReactNode;
  /** 下栏占容器高度的比例，0.1–0.8。 */
  ratio: number;
  /** 松开鼠标或按键结束时调用一次。 */
  onRatioChange: (ratio: number) => void;
  /** 上栏最小高度（rem），默认 6。 */
  minTopRem?: number;
  /** 下栏最小高度（rem），默认 5.5。 */
  minBottomRem?: number;
  /** 布局加载完成前为 true：不能拖、不能按键调整。 */
  disabled?: boolean;
}

const MIN_RATIO = 0.1;
const MAX_RATIO = 0.8;
const KEY_STEP = 0.05;

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));
const clampRatio = (v: number) => clamp(v, MIN_RATIO, MAX_RATIO);

/** 上下分栏，中间的分隔条可拖动（或用上下方向键）调整下栏占比。 */
export function SplitPane({
  top,
  bottom,
  ratio,
  onRatioChange,
  minTopRem = 6,
  minBottomRem = 5.5,
  disabled = false,
}: SplitPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [local, setLocal] = useState(() => clampRatio(ratio));
  const localRef = useRef(local);
  const onChangeRef = useRef(onRatioChange);
  onChangeRef.current = onRatioChange;

  const apply = useCallback((v: number) => {
    localRef.current = v;
    setLocal(v);
  }, []);

  // 外部 ratio 变化时同步
  useEffect(() => {
    apply(clampRatio(ratio));
  }, [ratio, apply]);

  /** 按容器高度和两栏最小高度换算可用的比例范围。 */
  const boundsFor = useCallback(
    (height: number) => {
      if (height <= 0) return { lo: MIN_RATIO, hi: MAX_RATIO };
      const lo = Math.max(MIN_RATIO, remToPx(minBottomRem) / height);
      const hi = Math.min(MAX_RATIO, 1 - remToPx(minTopRem) / height);
      return hi >= lo ? { lo, hi } : { lo: MIN_RATIO, hi: MAX_RATIO };
    },
    [minTopRem, minBottomRem],
  );

  const clampToContainer = useCallback(
    (v: number) => {
      const { lo, hi } = boundsFor(containerRef.current?.getBoundingClientRect().height ?? 0);
      return clamp(v, lo, hi);
    },
    [boundsFor],
  );

  const onPointerDown = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (disabled) return;
    e.preventDefault();
    const container = containerRef.current;
    if (!container) return;
    const onMove = (ev: PointerEvent) => {
      const rect = container.getBoundingClientRect();
      if (rect.height <= 0) return;
      const { lo, hi } = boundsFor(rect.height);
      apply(clamp((rect.bottom - ev.clientY) / rect.height, lo, hi));
    };
    const onUp = () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      window.removeEventListener('pointercancel', onUp);
      onChangeRef.current(localRef.current);
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
    window.addEventListener('pointercancel', onUp);
  };

  const onKeyDown = (e: ReactKeyboardEvent<HTMLDivElement>) => {
    if (disabled) return;
    // 向上移动分隔条 = 下栏变大
    const delta = e.key === 'ArrowUp' ? KEY_STEP : e.key === 'ArrowDown' ? -KEY_STEP : 0;
    if (delta === 0) return;
    e.preventDefault();
    const next = clampToContainer(localRef.current + delta);
    if (next === localRef.current) return;
    apply(next);
    onChangeRef.current(next);
  };

  return (
    <div ref={containerRef} className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-0 flex-1 flex-col">{top}</div>
      <div
        role="separator"
        aria-orientation="horizontal"
        aria-valuenow={Math.round(local * 100)}
        aria-valuemin={Math.round(MIN_RATIO * 100)}
        aria-valuemax={Math.round(MAX_RATIO * 100)}
        aria-disabled={disabled || undefined}
        tabIndex={0}
        title={disabled ? '布局未加载，暂时不能调整' : '拖动调整高度'}
        onPointerDown={onPointerDown}
        onKeyDown={onKeyDown}
        className={cn(
          'group flex h-2 shrink-0 items-center justify-center outline-none',
          disabled ? 'cursor-default' : 'cursor-row-resize',
        )}
      >
        <div
          className={cn(
            'h-0.5 w-12 rounded bg-border group-focus-visible:bg-primary',
            !disabled && 'group-hover:bg-primary',
          )}
        />
      </div>
      <div style={{ height: `${local * 100}%`, minHeight: `${minBottomRem}rem` }} className="shrink-0">
        {bottom}
      </div>
    </div>
  );
}
