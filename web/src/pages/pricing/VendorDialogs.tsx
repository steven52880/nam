import type { ReactNode } from 'react';
import type { VendorPricingItem } from '@/api/types';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { DASH, formatGroupRatio, formatNumber, formatPercent, formatRatioPercent } from '@/lib/format';
import { cnyPerUsd, currentPrice, rawRatioTitle, signedChange } from '@/lib/vendorPricing';


function InfoLine({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-baseline gap-2">
      <span className="w-24 shrink-0 text-muted-foreground">{label}</span>
      <span className="tabular-nums">{children}</span>
    </div>
  );
}

function PriceText({ price, usdRate }: { price: number | null; usdRate: number }) {
  if (price === null) return <>{DASH}</>;
  return (
    <>
      {formatRatioPercent(price)}
      <span className="ml-1 text-xs text-muted-foreground">≈ {formatNumber(cnyPerUsd(price, usdRate), 2)} 元/原厂 $1</span>
    </>
  );
}

interface DialogProps {
  row: VendorPricingItem | null;
  usdRate: number;
  protectThreshold: number;
  pending: boolean;
  onClose: () => void;
  onConfirm: (row: VendorPricingItem) => void;
}

/** 待确认：写清旧售价 → 新售价和变化幅度，确认后写入 NewAPI。 */
export function ConfirmPendingDialog({ row, usdRate, protectThreshold, pending, onClose, onConfirm }: DialogProps) {
  const newPrice = row ? (row.pending_price ?? row.target_price) : null;
  const change = row ? signedChange({ ...row, target_price: newPrice }) : null;
  return (
    <ConfirmDialog
      open={row !== null}
      onOpenChange={(open) => { if (!open) onClose(); }}
      title={`确认写入「${row?.vendor ?? ''}」的新售价`}
      description={`售价变化超过变化保护阈值（${formatRatioPercent(protectThreshold)}），确认后立即写入 NewAPI。`}
      confirmLabel="确认写入"
      pending={pending}
      onConfirm={() => { if (row) onConfirm(row); }}
    >
      {row && (
        <div className="grid gap-1">
          <InfoLine label="售价">
            <PriceText price={row.last_written_price} usdRate={usdRate} />
            <span className="mx-1.5 text-muted-foreground">→</span>
            <span className="font-medium"><PriceText price={newPrice} usdRate={usdRate} /></span>
          </InfoLine>
          <InfoLine label="变化">
            <span className="font-medium text-warning">{formatPercent(change, { signed: true })}</span>
          </InfoLine>
          <InfoLine label="厂商分组倍率">
            <span title={rawRatioTitle(row.newapi_ratio)}>{formatGroupRatio(row.newapi_ratio)}</span>
            <span className="mx-1.5 text-muted-foreground">→</span>
            <span className="font-medium">{formatGroupRatio(row.target_ratio)}</span>
          </InfoLine>
        </div>
      )}
    </ConfirmDialog>
  );
}

/** 漂移：以 nam 为准覆盖 NewAPI 里被改过的倍率（二次确认）。 */
export function ForceOverwriteDialog({ row, usdRate, pending, onClose, onConfirm }: Omit<DialogProps, 'protectThreshold'>) {
  return (
    <ConfirmDialog
      open={row !== null}
      onOpenChange={(open) => { if (!open) onClose(); }}
      title={`以 nam 为准覆盖「${row?.vendor ?? ''}」`}
      description="NewAPI 里这个分组的倍率被人改过。覆盖后会立即把它改回 nam 的目标倍率，别人改的值会丢失。"
      confirmLabel="确认覆盖"
      destructive
      pending={pending}
      onConfirm={() => { if (row) onConfirm(row); }}
    >
      {row && (
        <div className="grid gap-1">
          <InfoLine label="厂商分组倍率">
            <span className="text-destructive" title={rawRatioTitle(row.newapi_ratio)}>{formatGroupRatio(row.newapi_ratio)}</span>
            <span className="mx-1.5 text-muted-foreground">→</span>
            <span className="font-medium">{formatGroupRatio(row.target_ratio)}</span>
          </InfoLine>
          <InfoLine label="nam 售价"><PriceText price={currentPrice(row)} usdRate={usdRate} /></InfoLine>
          <InfoLine label="上次写入倍率">{formatGroupRatio(row.last_written_ratio)}</InfoLine>
        </div>
      )}
    </ConfirmDialog>
  );
}
