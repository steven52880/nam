import { Calculator, RefreshCw } from 'lucide-react';
import { useCallback, useDeferredValue, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { usePriceList, useVendorPricing } from '@/api/queries';
import type { PriceRow } from '@/api/types';
import { PageHeader } from '@/components/layout/PageHeader';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { Banner } from '@/components/ui/banner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ALL_FILTER, FilterSelect } from '@/components/filters/FilterSelect';
import { DataTable } from '@/components/table';
import { useActionMutation } from '@/hooks/useActionMutation';
import { canPreviewWrite } from '@/lib/vendorPricing';
import { cn } from '@/lib/utils';
import { AliasDialog } from './pricing/AliasDialog';
import { priceColumns, priceRowMenu } from './pricing/columns';
import { matchesTab, STATUS_TABS, tabCounts, type PriceTab } from './pricing/constants';
import { OverrideDialog } from './pricing/OverrideDialog';
import { PreviewWriteDialog } from './pricing/PreviewWriteDialog';
import { PriceDetailDrawer } from './pricing/PriceDetailDrawer';
import { useCalculatePricing } from './pricing/useVendorPricingActions';
import { VendorPricingSection } from './pricing/VendorPricingSection';

const getPriceRowId = (row: PriceRow) => row.model;

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
  const [status, setStatus] = useState<PriceTab>('all');
  const [vendor, setVendor] = useState(ALL_FILTER);
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search.trim());
  const [detailRow, setDetailRow] = useState<PriceRow | null>(null);
  const [overrideRow, setOverrideRow] = useState<PriceRow | null>(null);
  const [aliasRow, setAliasRow] = useState<PriceRow | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);

  // 状态页签在前端筛：后端的「偏差」行界面上算「正常」，让后端按状态筛会漏掉它们
  const list = usePriceList({
    vendor: vendor === ALL_FILTER ? undefined : vendor,
    q: deferredSearch || undefined,
  });
  const everything = usePriceList({});
  const sync = useSyncPrices();
  const vendorPricing = useVendorPricing();
  const calculate = useCalculatePricing();
  const canPreview = canPreviewWrite(vendorPricing.data);
  const previewHint = vendorPricing.data?.error
    ? 'NewAPI 的分组选项解析失败，不能写入'
    : '没有待写入的厂商，也没有分组选项变化';

  const vendors = useMemo(
    () => [...new Set((everything.data?.items ?? []).map((r) => r.vendor).filter((v) => v !== ''))].sort(),
    [everything.data],
  );
  const counts = useMemo(() => {
    const c = list.data?.counts ?? everything.data?.counts;
    return c ? tabCounts(c) : undefined;
  }, [list.data, everything.data]);
  const items = useMemo(() => (list.data?.items ?? []).filter((r) => matchesTab(r, status)), [list.data, status]);
  const vendorOptions = useMemo<Array<[string, string]>>(() => vendors.map((v) => [v, v]), [vendors]);
  const columns = useMemo(
    () => priceColumns({ onDetail: setDetailRow, onOverride: setOverrideRow, onAlias: setAliasRow }),
    [],
  );
  const rowMenu = useCallback(
    (row: PriceRow) => priceRowMenu(row, { onDetail: setDetailRow, onOverride: setOverrideRow, onAlias: setAliasRow }),
    [],
  );
  const tableStatus = useMemo(
    () => ({
      loading: list.isLoading,
      // 原来只在查询成功且为空时才显示这句；出错时上方有 Banner，表内留空
      empty: list.isSuccess ? '没有符合条件的模型，可以先点「同步价格库」' : '',
    }),
    [list.isLoading, list.isSuccess],
  );

  return (
    <div className="flex h-full flex-col gap-2 overflow-y-auto p-4">
      <PageHeader
        title="模型与价格"
        description="厂商定价决定写入 NewAPI 的分组倍率；NewAPI 里的模型价格保持原厂价，nam 不自动改，下表并列显示价格库价格和 NewAPI 当前价格。"
        actions={
          <>
            <Button variant="outline" disabled={sync.isPending} onClick={() => sync.mutate()}>
              <RefreshCw className={cn('h-3.5 w-3.5', sync.isPending && 'animate-spin')} />
              {sync.isPending ? '同步中…' : '同步价格库'}
            </Button>
            <Button variant="outline" disabled={calculate.isPending} onClick={() => calculate.mutate()} title="按当前渠道和设置重新计算所有厂商的目标售价">
              <Calculator className={cn('h-3.5 w-3.5', calculate.isPending && 'animate-pulse')} />
              {calculate.isPending ? '计算中…' : '立即计算'}
            </Button>
            <span title={canPreview ? undefined : previewHint}>
              <Button disabled={!canPreview} onClick={() => setPreviewOpen(true)}>预览写入</Button>
            </span>
          </>
        }
      />
      <VendorPricingSection />
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
        <FilterSelect className="w-36" label="厂商" value={vendor} options={vendorOptions} onChange={setVendor} />
        <Input className="w-56" placeholder="搜索模型名或别名" value={search} onChange={(e) => setSearch(e.target.value)} />
        <span className="text-sm text-muted-foreground"><ModelCountCell count={items.length} variant="text" /></span>
      </div>
      {list.isError && <Banner tone="error">{list.error.message}</Banner>}
      <DataTable<PriceRow>
        layoutId="pricing.list"
        className="min-h-72 flex-1"
        data={items}
        columns={columns}
        getRowId={getPriceRowId}
        status={tableStatus}
        onRowDoubleClick={setDetailRow}
        rowMenu={rowMenu}
      />
      <PriceDetailDrawer row={detailRow} onClose={() => setDetailRow(null)} />
      <OverrideDialog row={overrideRow} onClose={() => setOverrideRow(null)} />
      <AliasDialog row={aliasRow} onClose={() => setAliasRow(null)} />
      <PreviewWriteDialog open={previewOpen} onOpenChange={setPreviewOpen} />
    </div>
  );
}
