import type { UpstreamGroup } from '@/api/types';
import { numberCol, textCol, type TableColumn } from '@/components/table';
import { formatNumber, formatTime } from '@/lib/format';

/** 上游抽屉「分组」标签页的列；列宽单位 rem，列 id 上线后不改。 */
export function groupColumns(): TableColumn<UpstreamGroup>[] {
  return [
    textCol<UpstreamGroup>('name', '分组名', (g) => g.name, { width: 10, className: 'font-medium' }),
    numberCol<UpstreamGroup>('ratio', '分组倍率', (g) => g.ratio, {
      width: 6.5,
      cell: (g) => formatNumber(g.ratio, 3),
    }),
    textCol<UpstreamGroup>('desc', '说明', (g) => g.desc, {
      width: 16,
      className: 'text-muted-foreground',
      cell: (g) => g.desc || '—',
    }),
    textCol<UpstreamGroup>('updated_at', '更新时间', (g) => g.updated_at, {
      width: 11,
      sortType: 'number',
      className: 'tabular-nums',
      cell: (g) => formatTime(g.updated_at),
    }),
  ];
}
