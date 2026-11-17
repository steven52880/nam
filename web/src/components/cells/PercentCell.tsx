import { forwardRef, type HTMLAttributes } from 'react';
import { formatPercent } from '@/lib/format';
import { Dash } from './Dash';

interface PercentCellProps extends Omit<HTMLAttributes<HTMLSpanElement>, 'children'> {
  /** 0–1 的比率。 */
  value: number | null | undefined;
}

/** 百分比单元格：空值显示灰色「—」。可直接作为 TooltipTrigger asChild 的子元素。 */
export const PercentCell = forwardRef<HTMLSpanElement, PercentCellProps>(function PercentCell({ value, ...rest }, ref) {
  if (value === null || value === undefined) return <Dash />;
  return <span ref={ref} className="tabular-nums" {...rest}>{formatPercent(value)}</span>;
});
