import type { ReactNode, Ref } from 'react';
import type { MenuItem } from '@/components/ui/menu-items';
import type { TableLayoutId } from '@/lib/layout/slots';

/** 框选范围（从 data-grid/types.ts 复制；旧表格删除后以这里为准）。 */
export interface GridRange {
  rowIds: string[];
  columnIds: string[];
}

export interface SortSpec {
  id: string;
  desc: boolean;
}

export interface TableColumn<T> {
  /** 英文小写加下划线，上线后不改名；合成列以双下划线开头。 */
  id: string;
  header: string;
  /** 表头的 title 提示。 */
  headerTitle?: string;
  /** 默认列宽，rem；不传为 6.25。 */
  width?: number;
  /** 最小列宽，rem；默认 3。 */
  minWidth?: number;
  align?: 'left' | 'right';
  /** 单元格内容的附加 class（例如数字列的 tabular-nums），挂在 data-cell-content 上。 */
  className?: string;
  /** 用于排序和默认显示的值。 */
  accessor: (row: T) => string | number | null | undefined;
  /** 自定义渲染；不传则显示 accessor 的值。 */
  cell?: (row: T) => ReactNode;
  /** 进入编辑时的初始文本，也是复制时的默认文本；默认取 accessor。 */
  toText?: (row: T) => string;
  /** 默认 true。 */
  sortable?: boolean;
  /** 由列构造函数设置，默认 text。 */
  sortType?: 'text' | 'number';
  /** 默认隐藏。 */
  hidden?: boolean;
  /** 不能拖动、不能隐藏、不参与对账（操作列、合成列）。 */
  fixed?: boolean;
  // 以下只在 selection = 'cells' 时生效
  editable?: boolean;
  /** 只读列尝试编辑时 toast 的原因。 */
  readOnlyReason?: string;
  /** 返回错误信息表示校验失败，返回 null 表示通过。 */
  validate?: (value: string, row: T) => string | null;
  /** 提供后，双击 / Enter / F2 不进入行内编辑，而是调用它。 */
  openEditor?: (row: T) => void;
}

export interface TableLayout {
  /** 列 id → rem，只记用户调整过的列。 */
  widths: Record<string, number>;
  /** 不含 fixed 列。 */
  order: string[];
  /** 只记与默认不同的列。 */
  visibility: Record<string, boolean>;
  /** 用户排序；null = 使用 defaultSort。 */
  sort: SortSpec | null;
}

export interface DataTableProps<T> {
  layoutId: TableLayoutId;
  data: T[];
  columns: TableColumn<T>[];
  getRowId: (row: T) => string;
  /** 可多列，如 优先级↓ 权重↓ 名称↑。 */
  defaultSort?: SortSpec[];
  /** 服务端分页：不排序，表头不可点。 */
  manualSorting?: boolean;
  status?: { loading?: boolean; error?: string | null; empty?: ReactNode };
  /** 工具栏左侧；右侧固定是"列"按钮。 */
  toolbar?: ReactNode;
  className?: string;
  rowClassName?: (row: T) => string | undefined;
  cellClassName?: (row: T, columnId: string) => string | undefined;
  tableRef?: Ref<DataTableHandle<T>>;
  // 分组
  /** 返回组键；'' 表示兜底组。 */
  groupBy?: (row: T) => string;
  groupLabel?: (key: string) => ReactNode;
  sortGroups?: (keys: string[]) => string[];
  /** 变化时回到默认状态（默认折叠：全部折叠；groupsDefaultExpanded：全部展开）。 */
  groupResetKey?: string;
  /** 为 true 时分组默认全部展开（点组标题收起）；默认 false = 默认全部折叠。 */
  groupsDefaultExpanded?: boolean;
  /** 为 true 时全部展开（优先于手动收起，用于搜索）。 */
  expandAll?: boolean;
  /** 提供才允许拖动组标题；参数是可见组的新顺序。 */
  onGroupReorder?: (visibleKeys: string[]) => void;
  groupSummary?: (key: string, rows: T[]) => ReactNode;
  // 单元格框选与编辑
  /** 默认 none。 */
  selection?: 'cells' | 'none';
  onEdit?: (rowId: string, columnId: string, value: string) => void;
  onCopy?: (range: GridRange) => string | void;
  onPaste?: (range: GridRange, text: string) => void;
  isDirty?: (rowId: string, columnId: string) => boolean;
  onFocusRowChange?: (rowId: string | null) => void;
  /** 外部要求选中某一行（seq 变一次处理一次；这一行暂时不可见时等它出现）。 */
  focusRequest?: { rowId: string; columnId?: string; seq: number } | null;
  onSelectionChange?: (range: GridRange | null) => void;
  // 行级交互
  rowSelection?: {
    mode: 'single' | 'multi';
    selected: ReadonlySet<string>;
    onChange: (ids: Set<string>) => void;
    isDisabled?: (row: T) => boolean;
  };
  onRowClick?: (row: T) => void;
  onRowDoubleClick?: (row: T) => void;
  rowMenu?: (row: T) => MenuItem[];
  /** 行右键菜单顶部的灰色小标题（如渠道名）。 */
  rowMenuHeader?: (row: T) => ReactNode;
  renderExpanded?: (row: T) => ReactNode;
  /** 提供时禁用排序；参数是全部数据行的新顺序。 */
  onRowReorder?: (ids: string[]) => void;
  /** 例如保存中。 */
  rowReorderDisabled?: boolean;
}

export interface DataTableHandle<T> {
  /** 当前排序后的全部数据行（含折叠组里的）。 */
  getRows(): T[];
  resetLayout(): void;
  autoFitAll(): void;
}

/** 表格实际渲染的列：数据列，或表格内部生成的合成列（勾选、拖动手柄）。 */
export type DisplayCol<T> =
  | { kind: 'select'; id: '__select' }
  | { kind: 'drag'; id: '__drag' }
  | { kind: 'data'; id: string; col: TableColumn<T> };

/**
 * 扁平化后的表格条目。detail 与同一行的 row 使用相同的 id，
 * 渲染时用 `kind + id` 作为 key。
 */
export type TableItem<T> =
  | { kind: 'group'; key: string; rows: T[]; expanded: boolean }
  | { kind: 'row'; id: string; row: T }
  | { kind: 'detail'; id: string; row: T };
