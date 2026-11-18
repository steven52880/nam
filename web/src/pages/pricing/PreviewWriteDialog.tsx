import { ChevronDown, ChevronRight } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { useVendorPricingPreview } from '@/api/queries';
import type { VendorOptionDiff, VendorWriteChange, VendorWriteSkipped } from '@/api/types';
import { Banner } from '@/components/ui/banner';
import { Button } from '@/components/ui/button';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Badge, Switch } from '@/components/ui/misc';
import { useActionMutation } from '@/hooks/useActionMutation';
import { formatGroupRatio, formatRatioPercent } from '@/lib/format';
import { labelOf, VENDOR_PRICING_STATE } from '@/lib/labels';
import { prettyJson, rawRatioTitle } from '@/lib/vendorPricing';
import { applyToast } from './useVendorPricingActions';

function Arrow() {
  return <span className="mx-1 text-muted-foreground">→</span>;
}

function VendorChangeRow({ v }: { v: VendorWriteChange }) {
  return (
    <li className="flex items-center gap-3 border-t px-2 py-1 first:border-t-0">
      <span className="w-40 shrink-0 truncate">
        <span className="font-medium">{v.vendor}</span>
        <span className="ml-1.5 text-xs text-muted-foreground">{v.group}</span>
      </span>
      <span className="w-36 shrink-0 tabular-nums">
        <span className="mr-1 text-xs text-muted-foreground">售价</span>
        {formatRatioPercent(v.before_price)}<Arrow /><span className="font-medium">{formatRatioPercent(v.after_price)}</span>
      </span>
      <span className="tabular-nums">
        <span className="mr-1 text-xs text-muted-foreground">厂商分组倍率</span>
        <span title={rawRatioTitle(v.before_ratio)}>{formatGroupRatio(v.before_ratio)}</span>
        <Arrow />
        <span className="font-medium" title={rawRatioTitle(v.after_ratio)}>{formatGroupRatio(v.after_ratio)}</span>
      </span>
    </li>
  );
}

function SkippedRow({ s }: { s: VendorWriteSkipped }) {
  const def = labelOf(VENDOR_PRICING_STATE, s.state);
  return (
    <li className="flex items-center gap-2 border-t px-2 py-1 first:border-t-0">
      <span className="w-32 shrink-0 truncate font-medium">{s.vendor}</span>
      <Badge tone={def.tone}>{def.label}</Badge>
      <span className="min-w-0 flex-1 truncate text-muted-foreground" title={s.reason}>{s.reason}</span>
    </li>
  );
}

function OptionDiff({ o }: { o: VendorOptionDiff }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="rounded border">
      <button
        type="button"
        className="flex w-full items-center gap-1 bg-muted/50 px-2 py-1 text-left hover:bg-muted"
        onClick={() => setOpen(!open)}
      >
        {open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
        <span className="font-medium">{o.key}</span>
        <span className="text-sm text-muted-foreground">{open ? '收起前后对比' : '展开前后对比'}</span>
      </button>
      {open && (
        <div className="grid grid-cols-2 gap-px bg-border">
          <div className="min-w-0 bg-card">
            <div className="px-2 pt-1 text-xs text-muted-foreground">写入前</div>
            <pre className="max-h-64 overflow-auto px-2 pb-1 text-xs">{prettyJson(o.before)}</pre>
          </div>
          <div className="min-w-0 bg-card">
            <div className="px-2 pt-1 text-xs text-muted-foreground">写入后</div>
            <pre className="max-h-64 overflow-auto px-2 pb-1 text-xs">{prettyJson(o.after)}</pre>
          </div>
        </div>
      )}
    </div>
  );
}

function Block({ title, count, children }: { title: string; count?: number; children: ReactNode }) {
  return (
    <section className="grid gap-1">
      <h3 className="text-sm font-medium text-muted-foreground">
        {title}
        {count !== undefined && <span className="ml-1 tabular-nums">{count}</span>}
      </h3>
      {children}
    </section>
  );
}

/** 预览写入：先看会改什么，确认后才调 apply。 */
export function PreviewWriteDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [includeGroups, setIncludeGroups] = useState(true);
  const preview = useVendorPricingPreview(open, includeGroups);
  const apply = useActionMutation({
    mutationFn: () => api.applyVendorPricing({ include_groups: includeGroups }),
    invalidate: invalidate.vendorPricing,
    success: applyToast,
    onSuccess: () => onOpenChange(false),
  });

  const data = preview.data;
  const vendors = data?.vendors ?? [];
  const skipped = data?.skipped ?? [];
  const options = data?.options ?? [];
  const nothing = vendors.length === 0 && options.length === 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="3xl">
        <DialogHeader>
          <DialogTitle>预览写入</DialogTitle>
          <DialogDescription>以下内容将写入 NewAPI 的「GroupRatio」等选项，写入后会读回核对。</DialogDescription>
        </DialogHeader>
        <label className="flex items-center gap-2 text-sm">
          <Switch checked={includeGroups} onCheckedChange={setIncludeGroups} />
          同时更新 auto 分组和用户可用分组
        </label>
        {apply.isPending && (
          <div className="h-1.5 overflow-hidden rounded bg-muted">
            <div className="h-full w-1/3 animate-pulse rounded bg-primary" />
          </div>
        )}
        <DialogBody className="grid content-start gap-3">
          {preview.isLoading && <div className="py-6 text-center text-muted-foreground">计算中…</div>}
          {preview.isError && <Banner tone="error">{preview.error.message}</Banner>}
          {data && (
            <>
              <Block title="厂商倍率变化" count={vendors.length}>
                {vendors.length > 0 ? (
                  <ul className="rounded border text-base">{vendors.map((v) => <VendorChangeRow key={v.vendor_id} v={v} />)}</ul>
                ) : (
                  <div className="rounded border px-2 py-2 text-muted-foreground">没有厂商倍率需要写入</div>
                )}
              </Block>
              {skipped.length > 0 && (
                <Block title="跳过的厂商" count={skipped.length}>
                  <ul className="rounded border text-base">{skipped.map((s) => <SkippedRow key={s.vendor_id} s={s} />)}</ul>
                </Block>
              )}
              <Block title="NewAPI 选项变化" count={options.length}>
                {options.length > 0 ? (
                  <div className="grid gap-1.5">{options.map((o) => <OptionDiff key={o.key} o={o} />)}</div>
                ) : (
                  <div className="rounded border px-2 py-2 text-muted-foreground">没有选项变化</div>
                )}
              </Block>
            </>
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
          <Button disabled={!data || nothing || apply.isPending} onClick={() => apply.mutate()}>
            {apply.isPending ? '写入中…' : '确认写入'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
