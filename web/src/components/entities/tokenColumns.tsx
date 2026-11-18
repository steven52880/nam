import type { UpstreamGroup, UpstreamToken } from '@/api/types';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { numberCol, textCol, type TableColumn } from '@/components/table';
import { Badge } from '@/components/ui/misc';
import { formatNumber } from '@/lib/format';
import { TOKEN_STATUS } from '@/lib/labels';

export interface TokenColumnOptions {
  /** 该上游的分组倍率，用于「分组倍率」列；不传时该列为空。 */
  groups?: readonly UpstreamGroup[];
  /** 该上游的货币汇率，用于「渠道标价倍率」列（分组倍率 × 汇率）；不传时该列为空。 */
  rate?: number;
}

/** 上游页「令牌」标签页和渠道向导选令牌共用的列；列宽单位 rem，列 id 上线后不改。 */
export function tokenColumns({ groups, rate }: TokenColumnOptions = {}): TableColumn<UpstreamToken>[] {
  const ratioOf = (group: string): number | null => groups?.find((g) => g.name === group)?.ratio ?? null;
  const priceRatioOf = (group: string): number | null => {
    const ratio = ratioOf(group);
    return ratio === null || rate === undefined ? null : ratio * rate;
  };
  return [
    textCol<UpstreamToken>('name', '令牌名', (t) => t.name, {
      width: 11,
      cell: (t) => (
        <>
          <span className="font-medium">{t.name}</span>
          {t.manual && <Badge className="ml-1">手动</Badge>}
        </>
      ),
    }),
    textCol<UpstreamToken>('key_hint', 'key', (t) => t.key_hint, {
      width: 8,
      className: 'text-muted-foreground',
      cell: (t) => t.key_hint || '—',
    }),
    textCol<UpstreamToken>('group', '分组', (t) => t.group, { width: 7, cell: (t) => t.group || '—' }),
    numberCol<UpstreamToken>('group_ratio', '分组倍率', (t) => ratioOf(t.group), {
      width: 6.5,
      cell: (t) => formatNumber(ratioOf(t.group), 3),
    }),
    numberCol<UpstreamToken>('price_ratio', '渠道标价倍率', (t) => priceRatioOf(t.group), {
      width: 7,
      cell: (t) => formatNumber(priceRatioOf(t.group), 3),
    }),
    numberCol<UpstreamToken>('quota_usd', '额度（美元）', (t) => (t.unlimited ? null : t.quota_usd), {
      width: 8,
      cell: (t) => (t.unlimited ? '不限' : formatNumber(t.quota_usd)),
    }),
    textCol<UpstreamToken>('status', '状态', (t) => (t.status === 1 ? 1 : 0), {
      width: 5,
      sortType: 'number',
      cell: (t) => <StatusBadge dict={TOKEN_STATUS} value={t.status === 1 ? 1 : 0} />,
    }),
    textCol<UpstreamToken>('channel_count', '渠道', (t) => t.channel_count ?? 0, {
      width: 6,
      sortType: 'number',
      cell: (t) =>
        (t.channel_count ?? 0) > 0 ? (
          <Badge tone="accent">已建 {t.channel_count}</Badge>
        ) : (
          <span className="text-muted-foreground">未建</span>
        ),
    }),
  ];
}
