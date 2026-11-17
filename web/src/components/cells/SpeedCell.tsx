import { forwardRef, type HTMLAttributes } from 'react';
import { formatSpeed } from '@/lib/format';
import { Dash } from './Dash';

interface SpeedCellProps extends Omit<HTMLAttributes<HTMLSpanElement>, 'children'> {
  value: number | null | undefined;
  /** 为 true 时在数值后显示小号「估」。 */
  estimated?: boolean;
  /** 默认 true，显示「t/s」单位；false 只显示数字。 */
  unit?: boolean;
}

/** 吐字速度单元格：空值显示灰色「—」。可直接作为 TooltipTrigger asChild 的子元素。 */
export const SpeedCell = forwardRef<HTMLSpanElement, SpeedCellProps>(function SpeedCell(
  { value, estimated = false, unit = true, ...rest },
  ref,
) {
  if (value === null || value === undefined) return <Dash />;
  return (
    <span ref={ref} className="tabular-nums" {...rest}>
      {formatSpeed(value, { unit })}
      {estimated && <span className="ml-0.5 text-xs text-muted-foreground">估</span>}
    </span>
  );
});
