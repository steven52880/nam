import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { UpstreamToken } from '@/api/types';
import { ChipEditor } from '@/components/ChipEditor';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { uniq } from '@/lib/list';
import { vendorOf } from '@/pages/channels/wizard/StepModels';

function sameList(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((m, i) => m === b[i]);
}

interface FetchDialogProps {
  open: boolean;
  models: string[];
  current: string[];
  onClose: () => void;
  onConfirm: (picked: string[]) => void;
}

/** 从上游拉取到的模型勾选列表，可按厂商一键勾选。 */
function FetchDialog({ open, models, current, onClose, onConfirm }: FetchDialogProps) {
  const vendors = useQuery({ queryKey: ['vendors'], queryFn: api.vendors, enabled: open });
  const list = vendors.data ?? [];
  const [picked, setPicked] = useState<string[]>([]);

  useEffect(() => {
    if (open) setPicked(models.filter((m) => !current.includes(m)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, models]);

  const toggle = (m: string) => setPicked((cur) => (cur.includes(m) ? cur.filter((x) => x !== m) : [...cur, m]));
  const toggleVendor = (vendorId: number) => {
    const vendor = list.find((v) => v.id === vendorId);
    if (!vendor) return;
    const names = models.filter((m) => vendorOf(m, [vendor]) !== null);
    const all = names.length > 0 && names.every((m) => picked.includes(m));
    setPicked((cur) => (all ? cur.filter((m) => !names.includes(m)) : uniq([...cur, ...names])));
  };

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>从上游拉取的模型</DialogTitle>
          <DialogDescription>勾选要加入本站模型列表的模型，已在列表中的显示为灰色。</DialogDescription>
        </DialogHeader>
        <div className="grid gap-2">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-sm text-muted-foreground">按厂商勾选：</span>
            {list.map((v) => {
              const count = models.filter((m) => vendorOf(m, [v]) !== null).length;
              return (
                <Button key={v.id} size="sm" variant="outline" disabled={count === 0} onClick={() => toggleVendor(v.id)}>
                  {v.name}（{count}）
                </Button>
              );
            })}
            <Button size="sm" variant="outline" onClick={() => setPicked(models.filter((m) => !current.includes(m)))}>仅新增的</Button>
            <Button size="sm" variant="outline" onClick={() => setPicked([])}>清空</Button>
          </div>
          <div className="max-h-72 overflow-y-auto rounded border p-1">
            {models.map((m) => {
              const exists = current.includes(m);
              return (
                <label key={m} className="flex h-6 cursor-pointer items-center gap-2 rounded px-1 text-base hover:bg-accent/40">
                  <input type="checkbox" checked={picked.includes(m)} onChange={() => toggle(m)} />
                  <span className={exists ? 'text-muted-foreground' : undefined}>{m}</span>
                  {exists && <span className="text-xs text-muted-foreground">已有</span>}
                </label>
              );
            })}
            {models.length === 0 && <div className="py-6 text-center text-muted-foreground">上游没有返回模型</div>}
          </div>
          <div className="text-sm text-muted-foreground">已勾选 {picked.length} / {models.length} 个</div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button disabled={picked.length === 0} onClick={() => { onConfirm(picked); onClose(); }}>合并到列表</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** 令牌的"本站模型列表"编辑区：芯片编辑、从上游拉取、保存。 */
export function TokenModelsEditor({ upstreamId, token }: { upstreamId: number; token: UpstreamToken }) {
  const qc = useQueryClient();
  const saved = useMemo(() => token.models ?? [], [token.models]);
  const [models, setModels] = useState<string[]>(saved);
  const [fetched, setFetched] = useState<string[] | null>(null);

  useEffect(() => {
    setModels(saved);
  }, [saved]);

  const fetchModels = useMutation({
    mutationFn: () => api.fetchUpstreamModels(upstreamId, token.id),
    onSuccess: (res) => {
      toast.success(`已从上游拉取 ${res.models.length} 个模型`);
      setFetched(res.models);
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });
  const save = useMutation({
    mutationFn: () => api.setTokenModels(upstreamId, token.id, models),
    onSuccess: () => {
      toast.success('已保存令牌模型列表');
      void qc.invalidateQueries({ queryKey: ['upstream-tokens', upstreamId] });
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  const dirty = !sameList(models, saved);
  return (
    <div className="grid gap-2 bg-muted/30 p-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm text-muted-foreground">「{token.name}」要在本站开放的模型（共 {models.length} 个）</span>
        <Button size="sm" variant="outline" disabled={fetchModels.isPending} onClick={() => fetchModels.mutate()} title="用该令牌的 key 请求上游模型列表">
          {fetchModels.isPending ? '拉取中…' : '从上游拉取'}
        </Button>
        <Button size="sm" variant="outline" disabled={!dirty} onClick={() => setModels(saved)}>还原</Button>
        <Button size="sm" disabled={!dirty || save.isPending} onClick={() => save.mutate()}>{save.isPending ? '保存中…' : '保存'}</Button>
      </div>
      <ChipEditor value={models} onChange={setModels} />
      <FetchDialog
        open={fetched !== null}
        models={fetched ?? []}
        current={models}
        onClose={() => setFetched(null)}
        onConfirm={(picked) => setModels((cur) => uniq([...cur, ...picked]))}
      />
    </div>
  );
}
