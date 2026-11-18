import { useMemo } from 'react';
import { Combobox, type ComboboxItem } from '@/components/ui/combobox';

/** "全部" 选项的值；筛选状态等于它时表示不筛选。 */
export const ALL_FILTER = 'all';

export interface FilterSelectProps {
  /** 筛选项名称，第一项显示为"全部{label}"。 */
  label: string;
  value: string;
  /** [值, 显示文字]。 */
  options: Array<[string, string]>;
  onChange: (value: string) => void;
  /** 触发按钮的宽度等，默认 w-28。 */
  className?: string;
}

export function FilterSelect({ label, value, options, onChange, className = 'w-28' }: FilterSelectProps) {
  const items = useMemo<ComboboxItem[]>(
    () => [{ value: ALL_FILTER, label: `全部${label}` }, ...options.map(([v, l]) => ({ value: v, label: l }))],
    [label, options],
  );
  return (
    <Combobox
      variant="toolbar"
      items={items}
      value={value}
      onChange={(v) => onChange(v ?? ALL_FILTER)}
      searchable={items.length > 8}
      className={className}
    />
  );
}
