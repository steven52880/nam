import { useEffect, useState } from 'react';
import type { ChannelRow } from '@/api/types';
import { ChipEditor } from '@/components/ChipEditor';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';

interface ModelEditorDialogProps {
  row: ChannelRow | null;
  onClose: () => void;
  onSave: (row: ChannelRow, models: string[]) => void;
}

export function ModelEditorDialog({ row, onClose, onSave }: ModelEditorDialogProps) {
  const [models, setModels] = useState<string[]>([]);

  useEffect(() => {
    setModels(row?.models ?? []);
  }, [row]);

  return (
    <Dialog open={row !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>编辑模型</DialogTitle>
          <DialogDescription>
            {row?.name}。一个模型名只能属于一个厂商分组，保存时会校验。
          </DialogDescription>
        </DialogHeader>
        <ChipEditor value={models} onChange={setModels} />
        <div className="text-sm text-muted-foreground">共 {models.length} 个模型</div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button
            disabled={models.length === 0}
            onClick={() => {
              if (row) onSave(row, models);
              onClose();
            }}
          >
            保存到草稿
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
