import { RefreshCw } from 'lucide-react';
import { useDeferredValue, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { usePriceList } from '@/api/queries';
import type { PriceRow, PriceStatusFilter } from '@/api/types';
import { PageHeader } from '@/components/layout/PageHeader';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { Banner } from '@/components/ui/banner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { useActionMutation } from '@/hooks/useActionMutation';
import { cn } from '@/lib/utils';
import { AliasDialog } from './pricing/AliasDialog';
import { ALL_VENDORS, STATUS_TABS } from './pricing/constants';
import { OverrideDialog } from './pricing/OverrideDialog';
import { PriceDetailDrawer } from './pricing/PriceDetailDrawer';
import { PRICE_HEADERS, PriceTableRow } from './pricing/PriceTableRow';

function DisabledButton({ label }: { label: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span><Button variant="outline" disabled>{label}</Button></span>
      </TooltipTrigger>
      <TooltipContent>M5</TooltipContent>
    </Tooltip>
  );
}

function useSyncPrices() {
  return useActionMutation({
    mutationFn: api.syncPrices,
    invalidate: invalidate.prices,
    onSuccess: (result) => {
      if (result.last_result === 'fail') toast.error(`价格库同步失败：${result.last_message}`);
      else toast.success(result.last_message ? `价格库已同步：${result.last_message}` : '价格库已同步');
    },
  });
}

export default function PricingPage() {
  const [status, setStatus] = useState<PriceStatusFilter>('all');
  const [vendor, setVendor] = useState(ALL_VENDORS);
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search.trim());
  const [detailRow, setDetailRow] = useState<PriceRow | null>(null);
  const [overrideRow, setOverrideRow] = useState<PriceRow | null>(null);
  const [aliasRow, setAliasRow] = useState<PriceRow | null>(null);

  const list = usePriceList({
    status: status === 'all' ? undefined : status,
    vendor: vendor === ALL_VENDORS ? undefined : vendor,
    q: deferredSearch || undefined,
  });
  const everything = usePriceList({});
  const sync = useSyncPrices();

  const vendors = useMemo(
    () => [...new Set((everything.data?.items ?? []).map((r) => r.vendor).filter((v) => v !== ''))].sort(),
    [everything.data],
  );
  const counts = list.data?.counts ?? everything.data?.counts;
  const items = list.data?.items ?? [];
  const actions = { onDetail: setDetailRow, onOverride: setOverrideRow, onAlias: setAliasRow };

  return (
    <div className="flex h-full flex-col gap-2 p-4">
      <PageHeader
        title="模型与价格"
        description="NewAPI 里的模型价格保持原厂价，nam 不自动改；这里检查原厂价与 NewAPI 当前价的偏差。"
        actions={
          <>
            <Button variant="outline" disabled={sync.isPending} onClick={() => sync.mutate()}>
              <RefreshCw className={cn('h-3.5 w-3.5', sync.isPending && 'animate-spin')} />
              {sync.isPending ? '同步中…' : '同步价格库'}
            </Button>
            <DisabledButton label="立即计算" />
            <DisabledButton label="预览写入" />
          </>
        }
      />
      <div className="flex shrink-0 items-center justify-center rounded border border-dashed py-3 text-sm text-muted-foreground">
        售价与分组倍率在 M5 实现
      </div>
      <div className="flex shrink-0 flex-wrap items-center gap-2">
        <div className="flex gap-1">
          {STATUS_TABS.map((tab) => (
            <Button
              key={tab.value}
              size="sm"
              variant={status === tab.value ? 'default' : 'outline'}
              onClick={() => setStatus(tab.value)}
            >
              {tab.label}
              <span className="tabular-nums opacity-80">{counts ? counts[tab.value] : '—'}</span>
            </Button>
          ))}
        </div>
        <Select value={vendor} onValueChange={setVendor}>
          <SelectTrigger className="w-36"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_VENDORS}>全部厂商</SelectItem>
            {vendors.map((v) => <SelectItem key={v} value={v}>{v}</SelectItem>)}
          </SelectContent>
        </Select>
        <Input className="w-56" placeholder="搜索模型名或别名" value={search} onChange={(e) => setSearch(e.target.value)} />
        <span className="text-sm text-muted-foreground"><ModelCountCell count={items.length} variant="text" /></span>
      </div>
      {list.isError && <Banner tone="error">{list.error.message}</Banner>}
      <div className="min-h-0 flex-1 overflow-auto rounded border bg-card">
        <table className="w-full border-separate border-spacing-0 text-base">
          <thead className="sticky top-0 z-10 bg-muted">
            <tr>
              {PRICE_HEADERS.map((h, index) => (
                <th key={h} className={cn('h-7 whitespace-nowrap border-b border-r px-2 text-sm font-medium text-muted-foreground', index === 0 ? 'text-left' : 'text-left')}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {items.map((row) => <PriceTableRow key={row.model} row={row} actions={actions} />)}
            {list.isLoading && <tr><td colSpan={PRICE_HEADERS.length} className="h-16 text-center text-muted-foreground">加载中…</td></tr>}
            {list.isSuccess && items.length === 0 && (
              <tr><td colSpan={PRICE_HEADERS.length} className="h-16 text-center text-muted-foreground">没有符合条件的模型，可以先点「同步价格库」</td></tr>
            )}
          </tbody>
        </table>
      </div>
      <PriceDetailDrawer row={detailRow} onClose={() => setDetailRow(null)} />
      <OverrideDialog row={overrideRow} onClose={() => setOverrideRow(null)} />
      <AliasDialog row={aliasRow} onClose={() => setAliasRow(null)} />
    </div>
  );
}
