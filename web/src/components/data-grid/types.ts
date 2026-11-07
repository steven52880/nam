import type { ReactNode } from 'react';

export interface GridColumn<T> {
  id: string;
  header: string;
  width?: number;
  align?: 'left' | 'right';
  /** 用于排序和默认显示的值。 */
  accessor: (row: T) => string | number | null | undefined;
  /** 自定义渲染；不传则显示 accessor 的值。 */
  cell?: (row: T) => ReactNode;
  /** 进入编辑时的初始文本，也是复制时的默认文本；默认取 accessor。 */
  toText?: (row: T) => string;
  editable?: boolean;
  /** 只读列尝试编辑时 toast 的原因。 */
  readOnlyReason?: string;
  /** 返回错误信息表示校验失败，返回 null 表示通过。 */
  validate?: (value: string, row: T) => string | null;
  sortable?: boolean;
  /** 初始是否隐藏。 */
  hidden?: boolean;
}

export interface GridRange {
  rowIds: string[];
  columnIds: string[];
}
