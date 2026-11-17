import { MoreHorizontal } from 'lucide-react';
import type { PriceRow } from '@/api/types';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { NewapiPriceText, OfficialPriceText } from '@/components/PriceText';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Badge, Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatPercent } from '@/lib/format';
import { PRICE_STATUS } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { DEVIATION_THRESHOLD, sourceLabel } from './constants';

export const PRICE_HEADERS = ['模型', '原厂价（美元 / 百万）', '来源', 'NewAPI 当前价', '偏差', '渠道数', '状态', '操作'];

function Deviation({ row }: { row: PriceRow }) {
  if (!row.deviation) return <span className="text-muted-foreground">—</span>;
  const items: Array<[string, number | null]> = [
    ['按次', row.deviation.per_request],
    ['入', row.deviation.input],
    ['出', row.deviation.output],
    ['读', row.deviation.cache_read],
  ];
  if (items.every(([, value]) => value === null)) return <span className="text-muted-foreground">—</span>;
  return (
    <span className="inline-flex gap-2 tabular-nums">
      {items.map(([label, value]) => (
        <span key={label} className={cn(value !== null && Math.abs(value) > DEVIATION_THRESHOLD && 'font-medium text-destructive')}>
          <span className="text-xs text-muted-foreground">{label}</span> {formatPercent(value, { signed: true })}
        </span>
      ))}
    </span>
  );
}

interface RowActions {
  onDetail: (row: PriceRow) => void;
  onOverride: (row: PriceRow) => void;
  onAlias: (row: PriceRow) => void;
}

function Actions({ row, actions }: { row: PriceRow; actions: RowActions }) {
  return (
    <span className="inline-flex items-center gap-1">
      {row.status === 'deviation' && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span><Button size="sm" variant="outline" disabled>按建议执行</Button></span>
          </TooltipTrigger>
          <TooltipContent>M5 实现</TooltipContent>
        </Tooltip>
      )}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size="sm" variant="ghost">更多<MoreHorizontal className="h-3.5 w-3.5" /></Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onSelect={() => actions.onDetail(row)}>查看明细</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => actions.onOverride(row)}>覆盖原厂价</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => actions.onAlias(row)}>设置别名</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </span>
  );
}

export function PriceTableRow({ row, actions }: { row: PriceRow; actions: RowActions }) {
  return (
    <tr className="hover:bg-accent/40" onDoubleClick={() => actions.onDetail(row)}>
      <td className="border-b border-r px-2 py-1">
        <div className="flex items-center gap-1.5">
          <span className="font-medium">{row.model}</span>
          {row.vendor && <Badge>{row.vendor}</Badge>}
        </div>
        {row.aliases.length > 0 && (
          <div className="max-w-xs truncate text-xs text-muted-foreground" title={row.aliases.join('，')}>
            别名：{row.aliases.join('，')}
          </div>
        )}
      </td>
      <td className="border-b border-r px-2"><OfficialPriceText price={row.official} /></td>
      <td className="whitespace-nowrap border-b border-r px-2">{sourceLabel(row.source)}</td>
      <td className="border-b border-r px-2"><NewapiPriceText price={row.newapi} /></td>
      <td className="border-b border-r px-2"><Deviation row={row} /></td>
      <td className="border-b border-r px-2 text-right tabular-nums">{row.channel_count}</td>
      <td className="border-b border-r px-2"><StatusBadge dict={PRICE_STATUS} value={row.status} /></td>
      <td className="border-b px-2 text-right"><Actions row={row} actions={actions} /></td>
    </tr>
  );
}
