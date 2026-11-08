import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { PriceRow } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/input';
import { PRICES_KEY } from './constants';

function parseAliases(text: string): string[] {
  return [...new Set(text.split(/\r?\n/).map((line) => line.trim()).filter((line) => line !== ''))];
}

function AliasForm({ row, onDone }: { row: PriceRow; onDone: () => void }) {
  const qc = useQueryClient();
  const [text, setText] = useState(row.aliases.join('\n'));
  const save = useMutation({
    mutationFn: () => api.setAliases(row.model, parseAliases(text)),
    onSuccess: () => {
      toast.success('已保存别名（只写 nam）');
      void qc.invalidateQueries({ queryKey: PRICES_KEY });
      void qc.invalidateQueries({ queryKey: ['price-detail', row.model] });
      onDone();
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });
  return (
    <>
      <Textarea
        className="min-h-[140px] font-mono"
        placeholder="每行一个上游模型名"
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      <DialogFooter>
        <Button variant="outline" onClick={onDone}>取消</Button>
        <Button disabled={save.isPending} onClick={() => save.mutate()}>保存</Button>
      </DialogFooter>
    </>
  );
}

export function AliasDialog({ row, onClose }: { row: PriceRow | null; onClose: () => void }) {
  return (
    <Dialog open={row !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>设置别名：{row?.model}</DialogTitle>
          <DialogDescription>每行填一个上游模型名，映射到这个官方模型名。只写 nam。</DialogDescription>
        </DialogHeader>
        {row && <AliasForm key={row.model} row={row} onDone={onClose} />}
      </DialogContent>
    </Dialog>
  );
}
