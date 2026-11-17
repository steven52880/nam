import { usePriceDetail } from '@/api/queries';
import type { PriceRow } from '@/api/types';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { NewapiPriceText, OfficialPriceText } from '@/components/PriceText';
import { Dialog, DialogDescription, DialogTitle, SheetContent } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/misc';
import { formatNumber, formatSignificant } from '@/lib/format';
import { PRICE_STATUS } from '@/lib/labels';
import { sourceLabel } from './constants';

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="grid gap-1">
      <h3 className="text-sm font-medium text-muted-foreground">{title}</h3>
      {children}
    </section>
  );
}

function DetailBody({ row }: { row: PriceRow }) {
  const model = row.model;
  const query = usePriceDetail(model);
  if (query.isLoading) return <div className="p-4 text-muted-foreground">加载中…</div>;
  if (query.isError || !query.data) return <div className="p-4 text-destructive">{query.error?.message ?? '加载失败'}</div>;
  const { override, channels, aliases } = query.data;

  return (
    <div className="grid gap-4 overflow-y-auto p-4">
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge dict={PRICE_STATUS} value={row.status} />
        {row.vendor && <Badge>{row.vendor}</Badge>}
        <span className="text-sm text-muted-foreground">价格来源：{sourceLabel(row.source)}</span>
        {override && <Badge tone="warning">有手动覆盖</Badge>}
      </div>
      <Section title="价格对比（美元 / 百万）">
        <div className="grid gap-1 rounded border p-2">
          <div><span className="mr-2 text-sm text-muted-foreground">原厂价</span><OfficialPriceText price={row.official} /></div>
          <div><span className="mr-2 text-sm text-muted-foreground">NewAPI</span><NewapiPriceText price={row.newapi} /></div>
        </div>
      </Section>
      {aliases.length > 0 && (
        <Section title="别名">
          <div className="flex flex-wrap gap-1">{aliases.map((a) => <Badge key={a}>{a}</Badge>)}</div>
        </Section>
      )}
      {override?.remark && (
        <Section title="覆盖备注"><p className="text-sm">{override.remark}</p></Section>
      )}
      <Section title={`使用该模型的渠道（${channels.length}）`}>
        <div className="overflow-auto rounded border">
          <table className="w-full text-base">
            <thead className="bg-muted text-left text-sm text-muted-foreground">
              <tr>
                <th className="h-7 px-2 font-medium">渠道</th>
                <th className="px-2 text-right font-medium">上游分组折扣</th>
                <th className="px-2 text-right font-medium">成本（元 / 百万 输入）</th>
                <th className="px-2 text-right font-medium">成本（元 / 百万 输出）</th>
              </tr>
            </thead>
            <tbody>
              {channels.map((c) => (
                <tr key={c.id} className="h-7 border-t">
                  <td className="px-2">{c.name}</td>
                  <td className="px-2 text-right tabular-nums">{formatNumber(c.discount, 3)}</td>
                  <td className="px-2 text-right tabular-nums">{formatSignificant(c.cost_in_per_m)}</td>
                  <td className="px-2 text-right tabular-nums">{c.cost_per_request !== null ? `${formatSignificant(c.cost_per_request)} 元/次` : formatSignificant(c.cost_out_per_m)}</td>
                </tr>
              ))}
              {channels.length === 0 && (
                <tr><td colSpan={4} className="h-12 text-center text-muted-foreground">没有渠道使用该模型</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </Section>
    </div>
  );
}

export function PriceDetailDrawer({ row, onClose }: { row: PriceRow | null; onClose: () => void }) {
  return (
    <Dialog open={row !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      {row && (
        <SheetContent>
          <div className="border-b px-4 py-3">
            <DialogTitle>{row.model}</DialogTitle>
            <DialogDescription>模型价格明细与使用该模型的渠道</DialogDescription>
          </div>
          <DetailBody row={row} />
        </SheetContent>
      )}
    </Dialog>
  );
}
