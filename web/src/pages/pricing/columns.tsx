import { MoreHorizontal } from 'lucide-react';
import type { PriceRow } from '@/api/types';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { NewapiPriceText, OfficialPriceText } from '@/components/PriceText';
import { customCol, numberCol, textCol, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import type { MenuItem } from '@/components/ui/menu-items';
import { Badge, Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatPercent } from '@/lib/format';
import { labelOf, PRICE_STATUS } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { DEVIATION_THRESHOLD, sourceLabel } from './constants';

export interface RowActions {
  onDetail: (row: PriceRow) => void;
  onOverride: (row: PriceRow) => void;
  onAlias: (row: PriceRow) => void;
}

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

/** 行右键菜单：与"更多"下拉的三项相同。 */
export function priceRowMenu(row: PriceRow, actions: RowActions): MenuItem[] {
  return [
    { key: 'detail', label: '查看明细', onSelect: () => actions.onDetail(row) },
    { key: 'override', label: '覆盖原厂价', onSelect: () => actions.onOverride(row) },
    { key: 'alias', label: '设置别名', onSelect: () => actions.onAlias(row) },
  ];
}

/** 价格表的列。actions 要保持引用稳定（setState 或 useCallback）。 */
export function priceColumns(actions: RowActions): TableColumn<PriceRow>[] {
  return [
    textCol<PriceRow>('model', '模型', (r) => r.model, {
      width: 14,
      cell: (r) => (
        <span className="inline-flex items-center gap-1.5">
          <span className="font-medium">{r.model}</span>
          {r.vendor && <Badge>{r.vendor}</Badge>}
        </span>
      ),
    }),
    textCol<PriceRow>('aliases', '别名', (r) => r.aliases.join('，'), {
      width: 12,
      cell: (r) => (r.aliases.length > 0 ? <span title={r.aliases.join('，')}>{r.aliases.join('，')}</span> : null),
    }),
    customCol<PriceRow>('official', '原厂价（美元 / 百万）', {
      width: 19,
      cell: (r) => <OfficialPriceText price={r.official} />,
    }),
    textCol<PriceRow>('source', '来源', (r) => sourceLabel(r.source), { width: 6 }),
    customCol<PriceRow>('newapi', 'NewAPI 当前价', {
      width: 15,
      cell: (r) => <NewapiPriceText price={r.newapi} />,
    }),
    customCol<PriceRow>('deviation', '偏差', {
      width: 18,
      cell: (r) => <Deviation row={r} />,
    }),
    numberCol<PriceRow>('channel_count', '渠道数', (r) => r.channel_count, { width: 5.5 }),
    textCol<PriceRow>('status', '状态', (r) => labelOf(PRICE_STATUS, r.status).label, {
      width: 6.5,
      cell: (r) => <StatusBadge dict={PRICE_STATUS} value={r.status} />,
    }),
    customCol<PriceRow>('actions', '操作', {
      width: 11,
      align: 'right',
      fixed: true,
      cell: (r) => <Actions row={r} actions={actions} />,
    }),
  ];
}
