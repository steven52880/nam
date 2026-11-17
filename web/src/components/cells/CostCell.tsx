import type { ReactNode } from 'react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatNumber, formatSignificant } from '@/lib/format';
import { costStyle, type CostRange } from './costColor';
import { Dash } from './Dash';

interface CostCellProps {
  value: number | null | undefined;
  /** 当前可见行的成本范围，用于颜色梯度；null 不上色。 */
  range: CostRange | null;
  /** 默认 3；significant 时表示有效数字位数。 */
  digits?: number;
  /** 为 true 时按有效数字（formatSignificant）显示，否则按小数位数（formatNumber）。 */
  significant?: boolean;
  /** 悬停提示。 */
  hint?: ReactNode;
}

/** 成本单元格：按范围上色，空值显示灰色「—」。 */
export function CostCell({ value, range, digits = 3, significant = false, hint }: CostCellProps) {
  if (value === null || value === undefined) return <Dash />;
  const text = (
    <span className="tabular-nums" style={costStyle(value, range)}>
      {significant ? formatSignificant(value, digits) : formatNumber(value, digits)}
    </span>
  );
  if (!hint) return text;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{text}</TooltipTrigger>
      <TooltipContent>{hint}</TooltipContent>
    </Tooltip>
  );
}
