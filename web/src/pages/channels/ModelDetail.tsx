import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/endpoints';
import type { ChannelModels, ChannelRow, ModelRow } from '@/api/types';
import { OfficialPriceText } from '@/components/PriceText';
import { Badge } from '@/components/ui/misc';
import { formatNumber, formatPercent, formatSignificant } from '@/lib/format';
import { CacheOverridePopover, channelModelsKey, DiscountCell } from './ModelDiscountControls';

const HEADERS = [
  '模型名',
  '原厂价（美元 / 百万）',
  '上游价',
  '推算折扣',
  '上游分组折扣覆盖',
  '成本（元）',
  '请求数',
  '成功率',
  '首字延迟',
  '吐字速度',
];
const STATS_COLUMNS = 4;

function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

function DerivedCell({ row }: { row: ModelRow }) {
  if (row.derived === null) {
    return <span className="text-xs text-muted-foreground">{row.reason || '无法推算'}</span>;
  }
  return (
    <span className="inline-flex items-center justify-end gap-1 tabular-nums">
      {row.deviates && <Badge tone="warning" title={row.reason}>偏离 {formatPercent(row.deviation)}</Badge>}
      {formatNumber(row.derived, 3)}
    </span>
  );
}

function CostCell({ row }: { row: ModelRow }) {
  if (row.cost_per_request !== null) return <span className="tabular-nums">{formatSignificant(row.cost_per_request)} 元/次</span>;
  if (row.cost_in_per_m === null && row.cost_out_per_m === null) return <Dash />;
  return (
    <span className="tabular-nums">
      {formatSignificant(row.cost_in_per_m)} / {formatSignificant(row.cost_out_per_m)}
    </span>
  );
}

function Header({ detail }: { detail: ChannelModels }) {
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-0.5 border-b bg-muted/50 px-3 py-1 text-sm">
      <span className="font-medium">{detail.name}</span>
      <span className="text-muted-foreground">厂商：{detail.vendor || '—'}</span>
      <span className="inline-flex items-center gap-1 text-muted-foreground">
        上游分组折扣：{formatNumber(detail.discount, 3)}
        <Badge tone={detail.discount_source === 'manual' ? 'warning' : 'neutral'}>
          {detail.discount_source === 'manual' ? '手动' : '自动'}
        </Badge>
      </span>
      {detail.derived_median !== null && (
        <span className="text-muted-foreground">
          按当前上游价推算为 {formatNumber(detail.derived_median, 3)}，自动来源会在倍率同步后采用
        </span>
      )}
      <span className="text-muted-foreground">上游货币汇率：{formatNumber(detail.currency_rate)}</span>
      <span className="text-muted-foreground">测试模型：{detail.test_model || '—'}</span>
      <span className="ml-auto"><CacheOverridePopover channelId={detail.channel_id} /></span>
    </div>
  );
}

function useModelsData(channelId: number | null) {
  return useQuery({
    queryKey: channelModelsKey(channelId ?? 0),
    queryFn: () => api.channelModels(channelId ?? 0),
    enabled: channelId !== null && channelId > 0,
  });
}

export function ModelDetail({ row }: { row: ChannelRow | null }) {
  const query = useModelsData(row?.id ?? null);
  if (!row) {
    return (
      <div className="flex h-full items-center justify-center rounded border bg-card text-muted-foreground">
        点击上表中的渠道，查看它的模型明细
      </div>
    );
  }
  if (row.id < 0) {
    return (
      <div className="flex h-full items-center justify-center rounded border bg-card text-muted-foreground">
        新建草稿的渠道应用变更后才有模型明细
      </div>
    );
  }
  const detail = query.data;
  const models = detail?.models ?? [];
  return (
    <div className="flex h-full min-h-0 flex-col rounded border bg-card">
      {detail ? <Header detail={detail} /> : (
        <div className="shrink-0 border-b bg-muted/50 px-3 py-1 text-sm text-muted-foreground">
          {query.isError ? query.error.message : '加载中…'}
        </div>
      )}
      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-max min-w-full border-separate border-spacing-0 text-base">
          <thead className="sticky top-0 z-10 bg-muted">
            <tr>
              {HEADERS.map((h, index) => (
                <th key={h} className={`h-7 whitespace-nowrap border-b border-r px-2 text-sm font-medium text-muted-foreground ${index === 0 ? 'text-left' : 'text-right'}`}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {models.map((m) => (
              <tr key={m.model} className="hover:bg-accent/40">
                <td className="h-7 border-b border-r px-2">{m.model}</td>
                <td className="border-b border-r px-2 text-right"><OfficialPriceText price={m.price} /></td>
                <td className="border-b border-r px-2 text-right tabular-nums">{formatSignificant(m.upstream_input)}</td>
                <td className="border-b border-r px-2 text-right"><DerivedCell row={m} /></td>
                <td className="border-b border-r px-2 text-right">
                  <DiscountCell channelId={row.id} model={m.model} override={m.override} fallback={detail?.discount ?? row.discount} />
                </td>
                <td className="border-b border-r px-2 text-right"><CostCell row={m} /></td>
                {Array.from({ length: STATS_COLUMNS }, (_, i) => (
                  <td key={i} className="border-b border-r px-2 text-right"><Dash /></td>
                ))}
              </tr>
            ))}
            {detail && models.length === 0 && (
              <tr><td colSpan={HEADERS.length} className="h-12 text-center text-muted-foreground">该渠道没有模型</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
