import { KeyboardSensor, PointerSensor, useSensor, useSensors } from '@dnd-kit/core';
import { sortableKeyboardCoordinates, useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { CSSProperties } from 'react';

/** 指针移动多少距离才开始拖动（鼠标移动距离，不是界面尺寸）。 */
const DRAG_START_DISTANCE = 4;

/**
 * 传感器：指针移动 4 像素才开始 + 键盘（空格拿起、方向键移动、空格放下、Esc 取消）。
 * 内部用了 useSensor(s)，只能在组件函数体内调用。
 */
export function sortableSensors() {
  return useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: DRAG_START_DISTANCE } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
}

export interface SortableItemOptions {
  disabled?: boolean;
}

/** 封装 useSortable；`data.type` 用来区分拖动对象（column / group / row 等）。 */
export function useSortableItem(id: string, type: string, opts?: SortableItemOptions) {
  const s = useSortable({ id, data: { type }, disabled: opts?.disabled });
  const style: CSSProperties = {
    transform: CSS.Translate.toString(s.transform),
    transition: s.transition,
  };
  return {
    setNodeRef: s.setNodeRef,
    setActivatorNodeRef: s.setActivatorNodeRef,
    attributes: s.attributes,
    listeners: s.listeners,
    style,
    isDragging: s.isDragging,
    isOver: s.isOver,
    isSorting: s.isSorting,
    transform: s.transform,
    transition: s.transition,
  };
}
