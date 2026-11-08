import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { ChannelRow, DriftField } from '@/api/types';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useState } from 'react';
import { CHANNELS_KEY } from './useChannels';

const FIELD_LABELS: Record<string, string> = {
  models: '模型',
  group: '分组',
  priority: '优先级',
  weight: '权重',
  tag: '标签',
  status: '状态',
  test_model: '测试模型',
  base_url: '站点地址',
  name: '名称',
};

export function showValue(v: unknown): string {
  if (v === null || v === undefined || v === '') return '（空）';
  if (Array.isArray(v)) return v.join(', ') || '（空）';
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}

function sameValue(a: unknown, b: unknown): boolean {
  return showValue(a) === showValue(b);
}

function DriftRow({ field }: { field: DriftField }) {
  const changed = !sameValue(field.old, field.new);
  return (
    <tr className={changed ? 'bg-warning/10' : undefined}>
      <td className="border-b px-2 py-1 align-top font-medium">{FIELD_LABELS[field.field] ?? field.field}</td>
      <td className="max-w-56 break-all border-b px-2 py-1 align-top">{showValue(field.old)}</td>
      <td className="max-w-56 break-all border-b px-2 py-1 align-top">{showValue(field.new)}</td>
    </tr>
  );
}

interface DriftDialogProps {
  row: ChannelRow | null;
  onClose: () => void;
}

export function DriftDialog({ row, onClose }: DriftDialogProps) {
  const qc = useQueryClient();
  const [confirmOverwrite, setConfirmOverwrite] = useState(false);
  const id = row?.id ?? 0;
  const detail = useQuery({ queryKey: ['drift', id], queryFn: () => api.drift(id), enabled: row !== null });

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: CHANNELS_KEY });
    void qc.invalidateQueries({ queryKey: ['drift', id] });
  };
  const accept = useMutation({
    mutationFn: () => api.acceptDrift(id),
    onSuccess: () => { toast.success('已采纳 NewAPI 的值'); refresh(); onClose(); },
    onError: (e: Error) => toast.error(errorText(e)),
  });
  const overwrite = useMutation({
    mutationFn: () => api.overwriteDrift(id),
    onSuccess: () => {
      toast.success('已生成覆盖草稿，请到「应用变更」中确认写入 NewAPI');
      refresh();
      setConfirmOverwrite(false);
      onClose();
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  return (
    <>
      <Dialog open={row !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>漂移对比</DialogTitle>
            <DialogDescription>{row?.name}。左列是 nam 上次应用的值，右列是 NewAPI 当前值，不同的字段已标色。</DialogDescription>
          </DialogHeader>
          <div className="max-h-80 overflow-auto rounded border">
            <table className="w-full text-base">
              <thead className="bg-muted text-left text-sm text-muted-foreground">
                <tr><th className="px-2 py-1 font-medium">字段</th><th className="px-2 py-1 font-medium">nam</th><th className="px-2 py-1 font-medium">NewAPI 当前</th></tr>
              </thead>
              <tbody>
                {detail.data?.fields.map((f) => <DriftRow key={f.field} field={f} />)}
                {detail.isLoading && <tr><td colSpan={3} className="h-12 text-center text-muted-foreground">加载中…</td></tr>}
                {detail.isError && <tr><td colSpan={3} className="h-12 text-center text-destructive">{detail.error.message}</td></tr>}
              </tbody>
            </table>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={onClose}>稍后处理</Button>
            <Button variant="outline" disabled={accept.isPending} onClick={() => accept.mutate()}>采纳 NewAPI 值</Button>
            <Button variant="destructive" onClick={() => setConfirmOverwrite(true)}>以 nam 为准覆盖</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={confirmOverwrite}
        onOpenChange={setConfirmOverwrite}
        title="以 nam 为准覆盖"
        description="会生成一份草稿，把 NewAPI 的值改回 nam 记录的值。草稿需要在「应用变更」中确认后才会写入 NewAPI。"
        confirmLabel="生成覆盖草稿"
        destructive
        pending={overwrite.isPending}
        onConfirm={() => overwrite.mutate()}
      />
    </>
  );
}
