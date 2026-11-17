import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import type { ChannelRow } from '@/api/types';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useActionMutation } from '@/hooks/useActionMutation';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/misc';

export function TestChannelDialog({ row, onClose }: { row: ChannelRow | null; onClose: () => void }) {
  const [model, setModel] = useState('');

  useEffect(() => {
    if (row) setModel(row.test_model || row.models[0] || '');
  }, [row]);

  const run = useActionMutation({
    mutationFn: () => api.testChannel(row?.id ?? 0, model.trim()),
    onSuccess: (res) => {
      if (res.success) toast.success(`测试成功，用时 ${res.duration.toFixed(2)} 秒`);
      else toast.error(`测试失败：${res.message}`);
      onClose();
    },
  });

  return (
    <ConfirmDialog
      open={row !== null}
      onOpenChange={(open) => { if (!open) onClose(); }}
      title="测试渠道"
      description="测试会向上游发送一次真实请求，会产生少量费用。"
      confirmLabel={run.isPending ? '测试中…' : '开始测试'}
      pending={run.isPending}
      disabled={model.trim() === ''}
      onConfirm={() => run.mutate()}
    >
      <div className="grid gap-1">
        <Label>测试模型（默认为渠道的测试模型）</Label>
        <Input value={model} onChange={(e) => setModel(e.target.value)} list="channel-test-models" />
        <datalist id="channel-test-models">
          {row?.models.map((m) => <option key={m} value={m} />)}
        </datalist>
      </div>
    </ConfirmDialog>
  );
}
