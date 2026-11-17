import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { usePriceDetail } from '@/api/queries';
import type { PriceOverride, PriceRow } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useActionMutation } from '@/hooks/useActionMutation';

type PriceField = 'input' | 'output' | 'cache_read' | 'cache_write_5m' | 'cache_write_1h' | 'per_request';

const TOKEN_FIELDS: Array<[PriceField, string]> = [
  ['input', '输入'],
  ['output', '输出'],
  ['cache_read', '缓存读'],
  ['cache_write_5m', '缓存写（5 分钟）'],
  ['cache_write_1h', '缓存写（1 小时）'],
];

type FieldTexts = Record<PriceField, string>;

function textOf(value: number | null | undefined): string {
  return value === null || value === undefined ? '' : String(value);
}

function initialTexts(row: PriceRow, override: PriceOverride | null): FieldTexts {
  const source = override ?? row.official;
  return {
    input: textOf(source?.input),
    output: textOf(source?.output),
    cache_read: textOf(source?.cache_read),
    cache_write_5m: textOf(source?.cache_write_5m),
    cache_write_1h: textOf(source?.cache_write_1h),
    per_request: textOf(source?.per_request),
  };
}

function parsePrice(text: string): number | null | undefined {
  if (text.trim() === '') return null;
  const value = Number(text);
  return Number.isFinite(value) && value >= 0 ? value : undefined;
}

function OverrideForm({ row, override, onDone }: { row: PriceRow; override: PriceOverride | null; onDone: () => void }) {
  const [perRequest, setPerRequest] = useState(override?.per_request_mode ?? row.status === 'per_request');
  const [texts, setTexts] = useState<FieldTexts>(() => initialTexts(row, override));
  const [remark, setRemark] = useState(override?.remark ?? '');

  const save = useActionMutation({
    mutationFn: (body: PriceOverride) => api.setPriceOverride(row.model, body),
    success: '已保存覆盖（只写 nam）',
    invalidate: invalidate.prices,
    onSuccess: onDone,
  });
  const clear = useActionMutation({
    mutationFn: () => api.clearPriceOverride(row.model),
    success: '已清除覆盖',
    invalidate: invalidate.prices,
    onSuccess: onDone,
  });

  const submit = () => {
    const parsed = Object.fromEntries(
      (Object.keys(texts) as PriceField[]).map((field) => [field, parsePrice(texts[field])]),
    ) as Record<PriceField, number | null | undefined>;
    if (Object.values(parsed).some((value) => value === undefined)) {
      toast.error('价格必须是不小于 0 的数字');
      return;
    }
    const price = (field: PriceField) => parsed[field] ?? null;
    if (perRequest ? price('per_request') === null : price('input') === null || price('output') === null) {
      toast.error(perRequest ? '按次计费需要填写每次价格' : '按 token 计费至少需要填写输入和输出价格');
      return;
    }
    save.mutate({
      per_request_mode: perRequest,
      input: perRequest ? null : price('input'),
      output: perRequest ? null : price('output'),
      cache_read: perRequest ? null : price('cache_read'),
      cache_write_5m: perRequest ? null : price('cache_write_5m'),
      cache_write_1h: perRequest ? null : price('cache_write_1h'),
      per_request: perRequest ? price('per_request') : null,
      remark: remark.trim(),
    });
  };

  const setText = (field: PriceField, value: string) => setTexts((prev) => ({ ...prev, [field]: value }));
  const busy = save.isPending || clear.isPending;

  return (
    <>
      <div className="flex gap-2">
        <Button size="sm" variant={perRequest ? 'outline' : 'default'} onClick={() => setPerRequest(false)}>按 token</Button>
        <Button size="sm" variant={perRequest ? 'default' : 'outline'} onClick={() => setPerRequest(true)}>按次</Button>
      </div>
      {perRequest ? (
        <label className="grid gap-1 text-sm">每次价格（美元 / 次）
          <Input inputMode="decimal" value={texts.per_request} onChange={(e) => setText('per_request', e.target.value)} />
        </label>
      ) : (
        <div className="grid grid-cols-2 gap-2">
          {TOKEN_FIELDS.map(([field, label]) => (
            <label key={field} className="grid gap-1 text-sm">{label}（美元 / 百万）
              <Input inputMode="decimal" value={texts[field]} onChange={(e) => setText(field, e.target.value)} />
            </label>
          ))}
        </div>
      )}
      <label className="grid gap-1 text-sm">备注
        <Input value={remark} onChange={(e) => setRemark(e.target.value)} />
      </label>
      <DialogFooter className="justify-between">
        <Button variant="ghost" disabled={busy || override === null} onClick={() => clear.mutate()}>清除覆盖</Button>
        <Button disabled={busy} onClick={submit}>保存</Button>
      </DialogFooter>
    </>
  );
}

function OverrideBody({ row, onDone }: { row: PriceRow; onDone: () => void }) {
  const query = usePriceDetail(row.model);
  if (query.isLoading) return <div className="py-6 text-center text-muted-foreground">加载中…</div>;
  return <OverrideForm row={row} override={query.data?.override ?? null} onDone={onDone} />;
}

export function OverrideDialog({ row, onClose }: { row: PriceRow | null; onClose: () => void }) {
  return (
    <Dialog open={row !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>覆盖原厂价：{row?.model}</DialogTitle>
          <DialogDescription>只写 nam，不会修改 NewAPI 里的模型价格。</DialogDescription>
        </DialogHeader>
        {row && <OverrideBody row={row} onDone={onClose} />}
      </DialogContent>
    </Dialog>
  );
}
