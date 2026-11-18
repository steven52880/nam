import { usePriceDetail } from '@/api/queries';
import type { PriceChannelUse, PriceRow } from '@/api/types';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { NewapiPriceText, OfficialPriceText } from '@/components/PriceText';
import { DataTable, numberCol, textCol, type TableColumn } from '@/components/table';
import { DrawerShell } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/misc';
import { Section } from '@/components/ui/section';
import { formatNumber, formatSignificant } from '@/lib/format';
import { PRICE_STATUS } from '@/lib/labels';
import { sourceLabel } from './constants';

const CHANNEL_COLUMNS: TableColumn<PriceChannelUse>[] = [
  textCol('name', '渠道', (c) => c.name, { width: 12 }),
  numberCol('discount', '上游分组折扣', (c) => c.discount, {
    width: 8,
    cell: (c) => formatNumber(c.discount, 3),
  }),
  numberCol('cost_in', '成本（元 / 百万 输入）', (c) => c.cost_in_per_m, {
    width: 12,
    cell: (c) => formatSignificant(c.cost_in_per_m),
  }),
  numberCol('cost_out', '成本（元 / 百万 输出）', (c) => c.cost_per_request ?? c.cost_out_per_m, {
    width: 12,
    cell: (c) =>
      c.cost_per_request !== null ? `${formatSignificant(c.cost_per_request)} 元/次` : formatSignificant(c.cost_out_per_m),
  }),
];
const CHANNEL_STATUS = { empty: '没有渠道使用该模型' };
const getChannelId = (c: PriceChannelUse) => String(c.id);

function DetailBody({ row }: { row: PriceRow }) {
  const model = row.model;
  const query = usePriceDetail(model);
  if (query.isLoading) return <div className="p-4 text-muted-foreground">加载中…</div>;
  if (query.isError || !query.data) return <div className="p-4 text-destructive">{query.error?.message ?? '加载失败'}</div>;
  const { override, channels, aliases } = query.data;

  return (
    <div className="grid gap-4 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge dict={PRICE_STATUS} value={row.status} />
        {row.vendor && <Badge>{row.vendor}</Badge>}
        <span className="text-sm text-muted-foreground">价格来源：{sourceLabel(row.source)}</span>
        {override && <Badge tone="warning">有手动覆盖</Badge>}
      </div>
      <Section variant="plain" title="价格对比（美元 / 百万）">
        <div className="grid gap-1 rounded border p-2">
          <div><span className="mr-2 text-sm text-muted-foreground">原厂价</span><OfficialPriceText price={row.official} /></div>
          <div><span className="mr-2 text-sm text-muted-foreground">NewAPI</span><NewapiPriceText price={row.newapi} /></div>
        </div>
      </Section>
      {aliases.length > 0 && (
        <Section variant="plain" title="别名">
          <div className="flex flex-wrap gap-1">{aliases.map((a) => <Badge key={a}>{a}</Badge>)}</div>
        </Section>
      )}
      {override?.remark && (
        <Section variant="plain" title="覆盖备注"><p className="text-sm">{override.remark}</p></Section>
      )}
      <Section variant="plain" title={`使用该模型的渠道（${channels.length}）`}>
        <DataTable<PriceChannelUse>
          layoutId="pricing.channels"
          className="max-h-80"
          data={channels}
          columns={CHANNEL_COLUMNS}
          getRowId={getChannelId}
          status={CHANNEL_STATUS}
        />
      </Section>
    </div>
  );
}

export function PriceDetailDrawer({ row, onClose }: { row: PriceRow | null; onClose: () => void }) {
  return (
    <DrawerShell
      open={row !== null}
      onOpenChange={(open) => { if (!open) onClose(); }}
      title={row?.model}
      description="模型价格明细与使用该模型的渠道"
    >
      {row && <DetailBody row={row} />}
    </DrawerShell>
  );
}
