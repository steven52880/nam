import type { MouseEvent, ReactNode } from 'react';

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
  /** 提供后，双击 / Enter / F2 不进入行内编辑，而是调用它（例如打开弹窗编辑器）。 */
  openEditor?: (row: T) => void;
}

export interface GridRange {
  rowIds: string[];
  columnIds: string[];
}

export interface DataGridProps<T> {
  data: T[];
  columns: GridColumn<T>[];
  getRowId: (row: T) => string;
  /** 分组键；提供后按键分组并显示可折叠的组标题行。 */
  groupBy?: (row: T) => string;
  groupSummary?: (groupKey: string, rows: T[]) => ReactNode;
  isDirty?: (rowId: string, columnId: string) => boolean;
  /** 单元格提交（已通过校验）。 */
  onEdit?: (rowId: string, columnId: string, value: string) => void;
  /** Ctrl+C。返回文本时写入系统剪贴板。 */
  onCopy?: (range: GridRange) => string | void;
  /** Ctrl+V。text 为剪贴板文本。 */
  onPaste?: (range: GridRange, text: string) => void;
  toolbar?: ReactNode;
  className?: string;
  /** 行样式（例如弱化被禁用的行）。 */
  rowClassName?: (row: T) => string | undefined;
  /** 单元格附加样式（例如漂移高亮）。 */
  cellClassName?: (row: T, columnId: string) => string | undefined;
  /** 在行上右键。调用方负责显示菜单。 */
  onRowContextMenu?: (row: T, event: MouseEvent<HTMLTableRowElement>) => void;
  /** 焦点单元格所在行变化。 */
  onFocusRowChange?: (rowId: string | null) => void;
  /** 选区变化（用于批量操作）。 */
  onSelectionChange?: (range: GridRange | null) => void;
}
