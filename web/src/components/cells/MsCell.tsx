import { forwardRef, type HTMLAttributes } from 'react';
import { formatMs } from '@/lib/format';
import { Dash } from './Dash';

interface MsCellProps extends Omit<HTMLAttributes<HTMLSpanElement>, 'children'> {
  value: number | null | undefined;
}

/** 毫秒单元格（「850 ms」）：空值显示灰色「—」。可直接作为 TooltipTrigger asChild 的子元素。 */
export const MsCell = forwardRef<HTMLSpanElement, MsCellProps>(function MsCell({ value, ...rest }, ref) {
  if (value === null || value === undefined) return <Dash />;
  return <span ref={ref} className="tabular-nums" {...rest}>{formatMs(value)}</span>;
});
