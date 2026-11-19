import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent, type RefObject } from 'react';
import { cellsIn, rectangle, type CellRef, type Rectangle } from './machine';

export interface GridSelectionOptions {
  containerRef: RefObject<HTMLElement | null>;
  /** 当前可见行（已排序、已排除折叠分组）的 id。 */
  rowIds: string[];
  /** 当前可见列的 id。 */
  columnIds: string[];
}

const ARROWS = new Set(['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight']);

/**
 * 单元格范围选区：拖动框选、Shift+点击扩展、方向键移动焦点、Shift+方向键扩展、Esc 清除。
 * DOM 约定：每个数据单元格带 data-sheet-cell="true"、data-row-id、data-col-id。
 * 点击 input / button / textarea / select / label 不会开始选择。
 */
export function useGridSelection({ containerRef, rowIds, columnIds }: GridSelectionOptions) {
  const [anchor, setAnchor] = useState<CellRef | null>(null);
  const [focus, setFocus] = useState<CellRef | null>(null);
  const draggingRef = useRef(false);

  const rect = useMemo<Rectangle | null>(
    () => (anchor && focus ? rectangle(rowIds, columnIds, anchor, focus) : null),
    [anchor, focus, rowIds, columnIds],
  );

  const selected = useMemo(() => {
    const keys = new Set<string>();
    for (const cell of cellsIn(rect)) keys.add(`${cell.rowId}\u0000${cell.columnId}`);
    return keys;
  }, [rect]);

  const isSelected = useCallback(
    (rowId: string, columnId: string) => selected.has(`${rowId}\u0000${columnId}`),
    [selected],
  );

  const isFocus = useCallback(
    (rowId: string, columnId: string) => focus?.rowId === rowId && focus.columnId === columnId,
    [focus],
  );

  const clear = useCallback(() => {
    setAnchor(null);
    setFocus(null);
  }, []);

  /** 程序化选中单个单元格（如从提示条定位到某一行）。 */
  const select = useCallback((cell: CellRef) => {
    setAnchor(cell);
    setFocus(cell);
  }, []);

  const readCell = useCallback(
    (node: Element | null): CellRef | null => {
      const root = containerRef.current;
      if (!root || !node || !root.contains(node)) return null;
      const cell = node.closest("[data-sheet-cell='true']");
      if (!cell || !root.contains(cell)) return null;
      const rowId = cell.getAttribute('data-row-id');
      const columnId = cell.getAttribute('data-col-id');
      return rowId && columnId ? { rowId, columnId } : null;
    },
    [containerRef],
  );

  const onPointerDown = useCallback(
    (event: ReactPointerEvent<HTMLElement>) => {
      if (event.button !== 0 || !(event.target instanceof Element)) return;
      if (event.target.closest('input, button, textarea, select, label')) return;
      const cell = readCell(event.target);
      if (!cell) return;
      event.preventDefault();
      containerRef.current?.focus({ preventScroll: true });
      draggingRef.current = true;
      if (event.shiftKey && anchor) {
        setFocus(cell);
      } else {
        setAnchor(cell);
        setFocus(cell);
      }
    },
    [anchor, containerRef, readCell],
  );

  useEffect(() => {
    const onMove = (event: PointerEvent) => {
      if (!draggingRef.current) return;
      const cell = readCell(document.elementFromPoint(event.clientX, event.clientY));
      if (!cell) return;
      setFocus((cur) => (cur?.rowId === cell.rowId && cur.columnId === cell.columnId ? cur : cell));
    };
    const onUp = () => {
      draggingRef.current = false;
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
    return () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
    };
  }, [readCell]);

  // 键盘移动焦点时让焦点单元格滚入视野
  useEffect(() => {
    if (!focus || draggingRef.current) return;
    const root = containerRef.current;
    if (!root) return;
    for (const el of root.querySelectorAll("[data-sheet-cell='true']")) {
      if (el.getAttribute('data-row-id') === focus.rowId && el.getAttribute('data-col-id') === focus.columnId) {
        el.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        break;
      }
    }
  }, [focus, containerRef]);

  /** 处理方向键 / Esc，返回是否已处理。 */
  const handleKey = useCallback(
    (event: { key: string; shiftKey: boolean; preventDefault: () => void }): boolean => {
      if (event.key === 'Escape') {
        if (!anchor && !focus) return false;
        clear();
        event.preventDefault();
        return true;
      }
      if (!ARROWS.has(event.key) || rowIds.length === 0 || columnIds.length === 0) return false;
      event.preventDefault();
      const base: CellRef = focus ?? { rowId: rowIds[0], columnId: columnIds[0] };
      let r = Math.max(0, rowIds.indexOf(base.rowId));
      let c = Math.max(0, columnIds.indexOf(base.columnId));
      if (focus) {
        if (event.key === 'ArrowUp') r -= 1;
        else if (event.key === 'ArrowDown') r += 1;
        else if (event.key === 'ArrowLeft') c -= 1;
        else c += 1;
      }
      r = Math.min(rowIds.length - 1, Math.max(0, r));
      c = Math.min(columnIds.length - 1, Math.max(0, c));
      const next: CellRef = { rowId: rowIds[r], columnId: columnIds[c] };
      if (event.shiftKey && anchor) {
        setFocus(next);
      } else {
        setAnchor(next);
        setFocus(next);
      }
      return true;
    },
    [anchor, focus, rowIds, columnIds, clear],
  );

  return { rect, focus, isSelected, isFocus, clear, select, handleKey, onPointerDown };
}
