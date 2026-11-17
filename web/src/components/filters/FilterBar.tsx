import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';

export interface FilterBarProps {
  children: ReactNode;
  /** 点击"清除筛选"。 */
  onClear?: () => void;
  /** 当前是否有筛选条件；为 true 且有 onClear 时显示"清除筛选"。 */
  clearable: boolean;
  /** 排在"清除筛选"之后的内容（如渠道页右侧的统计时长，自行加 ml-auto）。 */
  trailing?: ReactNode;
}

export function FilterBar({ children, onClear, clearable, trailing }: FilterBarProps) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      {children}
      {clearable && onClear && <Button variant="link" onClick={onClear}>清除筛选</Button>}
      {trailing}
    </div>
  );
}
