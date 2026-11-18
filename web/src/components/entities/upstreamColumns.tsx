import type { Upstream } from '@/api/types';
import { numberCol, textCol, type TableColumn } from '@/components/table';
import { Badge } from '@/components/ui/misc';
import { formatNumber, formatTime } from '@/lib/format';

export interface UpstreamColumnOptions {
  /** 渠道向导里的写法：类型显示为纯文字，渠道数的表头为「已建渠道数」。 */
  wizard?: boolean;
}

/**
 * 上游页和渠道向导第一步共用的列。凭证、标签、错误、操作等只在上游页出现的列在 pages/upstreams 里。
 * 列宽单位 rem；列 id 上线后不改。
 */
export function upstreamColumns({ wizard = false }: UpstreamColumnOptions = {}): TableColumn<Upstream>[] {
  return [
    textCol<Upstream>('name', '名称', (u) => u.name, { width: 12, className: 'font-medium' }),
    textCol<Upstream>('base_url', '地址', (u) => u.base_url, { width: 14, className: 'text-xs text-muted-foreground' }),
    textCol<Upstream>('type', '类型', (u) => u.type, {
      width: 6,
      cell: wizard ? undefined : (u) => <Badge>{u.type}</Badge>,
    }),
    numberCol<Upstream>('currency_rate', '上游货币汇率（元/$）', (u) => u.currency_rate, {
      width: 9,
      headerTitle: '在上游充值 1 美元额度需要多少人民币，单位 元/$',
      cell: (u) => formatNumber(u.currency_rate),
    }),
    numberCol<Upstream>('balance_usd', '余额（美元）', (u) => u.balance_usd, {
      width: 8,
      cell: (u) => formatNumber(u.balance_usd),
    }),
    numberCol<Upstream>('balance_cny', '折合人民币', (u) => u.balance_cny, {
      width: 7.5,
      cell: (u) => formatNumber(u.balance_cny),
    }),
    numberCol<Upstream>('token_count', '令牌数', (u) => u.token_count, { width: 5 }),
    numberCol<Upstream>('channel_count', wizard ? '已建渠道数' : '渠道数', (u) => u.channel_count, {
      width: wizard ? 7 : 5,
    }),
    textCol<Upstream>('last_sync_at', '最近同步', (u) => u.last_sync_at, {
      width: 11,
      sortType: 'number',
      className: 'tabular-nums',
      cell: (u) => formatTime(u.last_sync_at),
    }),
  ];
}
