import type { ChannelRow } from '@/api/types';
import { Badge, Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatDiscount, formatGroupRatio } from '@/lib/format';
import { DISCOUNT_SOURCE } from '@/lib/labels';

/** 悬停说明：来源含义；有上游分组倍率时一并给出。 */
function hintOf(row: ChannelRow): string[] {
  const lines = [DISCOUNT_SOURCE[row.discount_source].hint ?? ''];
  if (row.discount_source === 'none') {
    if (row.discount_reason) lines.unshift(row.discount_reason);
    lines.push('可以手动填写：双击这一格输入数字');
  } else if (row.upstream_group_ratio !== null && row.discount_source !== 'group') {
    lines.push(`上游分组倍率：${formatGroupRatio(row.upstream_group_ratio)}`);
  }
  if (row.discount_source === 'manual') lines.push('清空后回到自动');
  return lines.filter(Boolean);
}

/** 渠道表「上游分组倍率」列（没选模型时）：数值 + 来源小标记；算不出显示「无法推算」。 */
export function ChannelDiscountCell({ row }: { row: ChannelRow }) {
  const def = DISCOUNT_SOURCE[row.discount_source];
  const none = row.discount === null;
  const content = (
    <span className="inline-flex items-center justify-end gap-1">
      {none ? (
        <span className="text-muted-foreground">{def.label}</span>
      ) : (
        <>
          <Badge tone={def.tone}>{def.label}</Badge>
          <span className="tabular-nums">{formatDiscount(row.discount as number)}</span>
        </>
      )}
    </span>
  );
  return (
    <Tooltip>
      <TooltipTrigger asChild>{content}</TooltipTrigger>
      <TooltipContent>
        {hintOf(row).map((line) => <div key={line}>{line}</div>)}
      </TooltipContent>
    </Tooltip>
  );
}
