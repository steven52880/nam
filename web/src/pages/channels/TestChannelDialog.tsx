import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import type { ChannelRow } from '@/api/types';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useActionMutation } from '@/hooks/useActionMutation';
import { Field } from '@/components/form/Field';
import { Combobox, type ComboboxItem } from '@/components/ui/combobox';

export function TestChannelDialog({ row, onClose }: { row: ChannelRow | null; onClose: () => void }) {
  const [model, setModel] = useState('');
  const items = useMemo<ComboboxItem[]>(() => (row?.models ?? []).map((m) => ({ value: m, label: m })), [row]);

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
      <Field label="测试模型（默认为渠道的测试模型）">
        <Combobox
          variant="field"
          allowCustom
          items={items}
          value={model === '' ? null : model}
          onChange={(v) => setModel(v ?? '')}
          placeholder="选择或输入模型"
        />
      </Field>
    </ConfirmDialog>
  );
}
