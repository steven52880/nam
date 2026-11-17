import { useState } from 'react';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import type { PriceRow } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/input';
import { useActionMutation } from '@/hooks/useActionMutation';

function parseAliases(text: string): string[] {
  return [...new Set(text.split(/\r?\n/).map((line) => line.trim()).filter((line) => line !== ''))];
}

function AliasForm({ row, onDone }: { row: PriceRow; onDone: () => void }) {
  const [text, setText] = useState(row.aliases.join('\n'));
  const save = useActionMutation({
    mutationFn: () => api.setAliases(row.model, parseAliases(text)),
    success: '已保存别名（只写 nam）',
    invalidate: invalidate.prices,
    onSuccess: onDone,
  });
  return (
    <>
      <DialogBody>
        <Textarea
          className="min-h-36 font-mono"
          placeholder="每行一个上游模型名"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </DialogBody>
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
