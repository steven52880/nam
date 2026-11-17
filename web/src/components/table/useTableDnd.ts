import {
  closestCenter,
  MeasuringStrategy,
  type CollisionDetection,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { restrictToHorizontalAxis, restrictToVerticalAxis } from '@dnd-kit/modifiers';
import { arrayMove } from '@dnd-kit/sortable';
import { useCallback, useEffect, useRef, useState } from 'react';
import { sortableSensors } from '@/components/dnd/Sortable';

export type DragType = 'column' | 'group' | 'row';

/** 三种拖动对象的 id 前缀：同一个 DndContext 里不同类型的 id 不会冲突。 */
const COL = 'col:';
const GRP = 'grp:';
const ROW = 'row:';
export const colSortId = (id: string) => `${COL}${id}`;
export const groupSortId = (key: string) => `${GRP}${key}`;
export const rowSortId = (id: string) => `${ROW}${id}`;

/** 去掉 sortable id 的类型前缀，取回原来的列 id / 组键 / 行 id。 */
export const sortIdValue = (sortId: string) => sortId.slice(sortId.indexOf(':') + 1);

const MODIFIERS_H = [restrictToHorizontalAxis];
const MODIFIERS_V = [restrictToVerticalAxis];

/** 只在同类型的可放置元素之间做碰撞检测（列只和列比，组只和组比）。 */
const sameTypeCollision: CollisionDetection = (args) => {
  const type = args.active.data.current?.type;
  return closestCenter({
    ...args,
    droppableContainers: args.droppableContainers.filter((c) => c.data.current?.type === type),
  });
};

/** id 列表用函数传入，拖动结束时才读取最新值（组折叠状态依赖本 hook 的 collapseAll，避免循环依赖）。 */
export interface TableDndArgs {
  /** 可拖动的可见列 id（非 fixed），按显示顺序。 */
  getColumnIds: () => string[];
  /** 拖完后的可见列新顺序。 */
  onColumnReorder: (visibleIds: string[]) => void;
  /** 当前显示的全部组键，按显示顺序。 */
  getGroupKeys: () => string[];
  /** 拖完后的组新顺序；不提供则不处理组拖动。 */
  onGroupReorder?: (visibleKeys: string[]) => void;
  /** 全部数据行 id，按显示顺序。 */
  getRowIds: () => string[];
  onRowReorder?: (ids: string[]) => void;
}

function reorder(ids: string[], from: string, to: string): string[] | null {
  const a = ids.indexOf(from);
  const b = ids.indexOf(to);
  if (a < 0 || b < 0 || a === b) return null;
  return arrayMove(ids, a, b);
}

/** 整张表一个 DndContext 用到的全部状态和处理函数。 */
export function useTableDnd(args: TableDndArgs) {
  const sensors = sortableSensors();
  const argsRef = useRef(args);
  argsRef.current = args;
  const [active, setActive] = useState<{ id: string; type: DragType } | null>(null);
  /** 刚拖完（或刚拖完列宽）的标记：同一次操作产生的 click 不触发排序、行点击。 */
  const justDragged = useRef(false);

  /**
   * 当前这次拖动的指针状态。
   * - pointer：由指针发起（键盘拖动没有 click，不屏蔽下一次真实点击）。
   * - held：指针还没释放。Esc 取消时用户可能还按着鼠标，之后松手产生的 click 要屏蔽。
   * - ended：拖动已经结束（放下或取消）。
   */
  const drag = useRef({ pointer: false, held: false, ended: false });
  const removeRelease = useRef<(() => void) | null>(null);

  const clearSoon = useCallback(() => {
    requestAnimationFrame(() => {
      justDragged.current = false;
    });
  }, []);

  useEffect(() => () => removeRelease.current?.(), []);

  /** 指针拖动开始：在 window 捕获阶段监听释放（早于 dnd-kit 在 document 上的监听）。 */
  const watchPointerRelease = useCallback(() => {
    removeRelease.current?.();
    const st = drag.current;
    st.pointer = true;
    st.held = true;
    st.ended = false;
    const onRelease = () => {
      st.held = false;
      removeRelease.current?.();
      // 拖动已经结束（Esc 取消后才松手）：屏蔽到这次释放产生的 click 之后
      if (st.ended) clearSoon();
    };
    window.addEventListener('pointerup', onRelease, true);
    window.addEventListener('pointercancel', onRelease, true);
    removeRelease.current = () => {
      window.removeEventListener('pointerup', onRelease, true);
      window.removeEventListener('pointercancel', onRelease, true);
      removeRelease.current = null;
    };
  }, [clearSoon]);

  /** 拖动结束（放下或取消）。 */
  const markDragged = useCallback(() => {
    const st = drag.current;
    st.ended = true;
    if (!st.pointer) return;
    justDragged.current = true;
    // 指针已经释放（正常放下）：屏蔽到这次 click 之后；还按着（Esc 取消）：等释放后再清除
    if (!st.held) clearSoon();
  }, [clearSoon]);

  /** 按下改宽手柄时调用：松开鼠标后的下一个任务才恢复，吃掉松开时可能冒出来的 click。 */
  const suppressUntilRelease = useCallback(() => {
    justDragged.current = true;
    const release = () => {
      window.removeEventListener('mouseup', release);
      window.removeEventListener('touchend', release);
      setTimeout(() => {
        justDragged.current = false;
      }, 0);
    };
    window.addEventListener('mouseup', release);
    window.addEventListener('touchend', release);
  }, []);

  const onDragStart = (e: DragStartEvent) => {
    const type = e.active.data.current?.type;
    if (type !== 'column' && type !== 'group' && type !== 'row') return;
    justDragged.current = false;
    if (e.activatorEvent.type === 'keydown') {
      removeRelease.current?.();
      drag.current = { pointer: false, held: false, ended: false };
    } else {
      watchPointerRelease();
    }
    setActive({ id: String(e.active.id), type });
  };

  const finish = () => {
    setActive(null);
    markDragged();
  };

  const onDragEnd = (e: DragEndEvent) => {
    const type = e.active.data.current?.type;
    const { over } = e;
    finish();
    if (!over || e.active.id === over.id) return;
    const from = String(e.active.id);
    const to = String(over.id);
    const a = argsRef.current;
    if (type === 'column') {
      const next = reorder(a.getColumnIds().map(colSortId), from, to);
      if (next) a.onColumnReorder(next.map((id) => id.slice(COL.length)));
    } else if (type === 'group') {
      const next = reorder(a.getGroupKeys().map(groupSortId), from, to);
      if (next) a.onGroupReorder?.(next.map((id) => id.slice(GRP.length)));
    } else if (type === 'row') {
      const next = reorder(a.getRowIds().map(rowSortId), from, to);
      if (next) a.onRowReorder?.(next.map((id) => id.slice(ROW.length)));
    }
  };

  /** Esc 取消：不改变任何顺序。 */
  const onDragCancel = () => finish();

  const draggingGroup = active?.type === 'group';
  return {
    sensors,
    active,
    /** 拖分组时全部折叠。 */
    collapseAll: draggingGroup,
    justDragged,
    suppressUntilRelease,
    collisionDetection: sameTypeCollision,
    modifiers: active?.type === 'column' ? MODIFIERS_H : MODIFIERS_V,
    // 拖分组时行数会变化，需要一直重新测量
    measuring: {
      droppable: { strategy: draggingGroup ? MeasuringStrategy.Always : MeasuringStrategy.WhileDragging },
    },
    onDragStart,
    onDragEnd,
    onDragCancel,
  };
}
