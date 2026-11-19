import type { ReactNode } from 'react';
import type { VendorPricingItem } from '@/api/types';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { DASH, formatGroupRatio, formatNumber, formatPercent, formatRatioPercent } from '@/lib/format';
import { cnyPerUsd, rawRatioTitle, signedChange } from '@/lib/vendorPricing';


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

/** 待确认：写清新售价、相对 NewAPI 当前售价的变化幅度和倍率变化，确认后写入 NewAPI。 */
export function ConfirmPendingDialog({ row, usdRate, protectThreshold, pending, onClose, onConfirm }: DialogProps) {
  const change = row ? signedChange(row) : null;
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
          <InfoLine label="新售价">
            <span className="font-medium"><PriceText price={row.target_price} usdRate={usdRate} /></span>
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
