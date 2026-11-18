import { useCallback, useMemo, useState } from 'react';
import { useVendorPricing } from '@/api/queries';
import type { VendorPricingItem } from '@/api/types';
import { DataTable } from '@/components/table';
import { Banner } from '@/components/ui/banner';
import { Section } from '@/components/ui/section';
import { formatNumber, formatRatioPercent } from '@/lib/format';
import { ConfirmPendingDialog, ForceOverwriteDialog } from './VendorDialogs';
import { vendorColumns, type VendorRowActions } from './VendorColumns';
import { useVendorPricingActions } from './useVendorPricingActions';

const getVendorId = (r: VendorPricingItem) => String(r.vendor_id);
const NO_ITEMS: VendorPricingItem[] = [];

function Param({ label, children }: { label: string; children: string }) {
  return (
    <span>
      {label} <span className="text-foreground tabular-nums">{children}</span>
    </span>
  );
}

/** 模型与价格页顶部的「厂商定价」区。 */
export function VendorPricingSection() {
  const q = useVendorPricing();
  const actions = useVendorPricingActions();
  const [confirmRow, setConfirmRow] = useState<VendorPricingItem | null>(null);
  const [overrideRow, setOverrideRow] = useState<VendorPricingItem | null>(null);

  const o = q.data;
  const usdRate = o?.usd_rate ?? 0;

  const { confirm, adopt, override } = actions;
  const confirmMutate = confirm.mutate;
  const adoptMutate = adopt.mutate;
  const overrideMutate = override.mutate;
  const busyId = confirm.isPending
    ? confirm.variables
    : adopt.isPending
      ? adopt.variables
      : override.isPending
        ? override.variables
        : null;

  const onAdopt = useCallback((row: VendorPricingItem) => adoptMutate(row.vendor_id), [adoptMutate]);
  const columnActions = useMemo<VendorRowActions>(
    () => ({
      usdRate,
      busyId,
      onMode: actions.setMode,
      onMarkup: actions.setMarkup,
      onSellPrice: actions.setSellPrice,
      onGroupRatio: actions.setGroupRatio,
      onConfirm: setConfirmRow,
      onAdopt,
      onOverride: setOverrideRow,
    }),
    [usdRate, busyId, actions.setMode, actions.setMarkup, actions.setSellPrice, actions.setGroupRatio, onAdopt],
  );
  const columns = useMemo(() => vendorColumns(columnActions), [columnActions]);
  const tableStatus = useMemo(
    () => ({
      loading: q.isLoading,
      error: q.isError ? q.error.message : null,
      empty: '还没有厂商，请先到「设置 → 厂商分组」添加',
    }),
    [q.isLoading, q.isError, q.error],
  );

  return (
    <Section
      title="厂商定价"
      flush
      description={
        o ? (
          <span className="inline-flex flex-wrap gap-x-3">
            <Param label="美元参考汇率">{formatNumber(o.usd_rate, 2, { trim: true })}</Param>
            <Param label="站点汇率">{formatNumber(o.site_rate, 2, { trim: true })}</Param>
            <Param label="变化保护阈值">{formatRatioPercent(o.protect_threshold)}</Param>
            <Param label="最小变动">{formatRatioPercent(o.min_change)}</Param>
            <Param label="自动写入">{o.auto_write ? '开' : '关'}</Param>
          </span>
        ) : undefined
      }
      className="shrink-0"
    >
      {o?.error && (
        <Banner tone="error" className="m-2">
          NewAPI 的分组选项解析失败，暂时不能写入：{o.error}
        </Banner>
      )}
      <DataTable<VendorPricingItem>
        layoutId="pricing.vendors"
        className="max-h-72 rounded-none border-0"
        data={o?.items ?? NO_ITEMS}
        columns={columns}
        getRowId={getVendorId}
        status={tableStatus}
        rowClassName={rowClass}
      />
      <ConfirmPendingDialog
        row={confirmRow}
        usdRate={usdRate}
        protectThreshold={o?.protect_threshold ?? 0}
        pending={confirm.isPending}
        onClose={() => setConfirmRow(null)}
        onConfirm={(row) => confirmMutate(row.vendor_id, { onSuccess: () => setConfirmRow(null) })}
      />
      <ForceOverwriteDialog
        row={overrideRow}
        usdRate={usdRate}
        pending={override.isPending}
        onClose={() => setOverrideRow(null)}
        onConfirm={(row) => overrideMutate(row.vendor_id, { onSuccess: () => setOverrideRow(null) })}
      />
    </Section>
  );
}

function rowClass(r: VendorPricingItem): string | undefined {
  if (r.state === 'drift') return 'bg-destructive/5';
  if (r.state === 'pending') return 'bg-warning/10';
  return undefined;
}
