import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

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
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className={className}><SelectValue /></SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL_FILTER}>全部{label}</SelectItem>
        {options.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
      </SelectContent>
    </Select>
  );
}
