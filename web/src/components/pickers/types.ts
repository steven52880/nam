export interface EntityPickerProps<V> {
  value: V | null;
  onChange: (value: V) => void;
  variant?: 'field' | 'cell' | 'toolbar';
  disabled?: boolean;
  placeholder?: string;
  title?: string;
  /** 传给 Combobox 的 className，用来覆盖触发器宽度等。 */
  className?: string;
  /** 列表还没加载、或当前值不在列表里时，触发器显示的文字。 */
  fallbackLabel?: string;
}
