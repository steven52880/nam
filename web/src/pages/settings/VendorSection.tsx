import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { Vendor, VendorInput } from '@/api/types';
import { ChipEditor } from '@/components/ChipEditor';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/misc';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { parseStringArray } from '@/lib/list';

const VENDORS_KEY = ['vendors'] as const;
const TYPE_HELP = '透传（58）适用于 OpenAI / Claude 格式；Gemini 原生格式的渠道必须选 24，否则计费不正确。';

interface VendorForm {
  id: number | null;
  name: string;
  groupName: string;
  channelType: 58 | 24;
  models: string[];
}

const EMPTY_FORM: VendorForm = { id: null, name: '', groupName: '', channelType: 58, models: [] };

function VendorDialog({ form, onClose }: { form: VendorForm | null; onClose: () => void }) {
  const qc = useQueryClient();
  const [draft, setDraft] = useState<VendorForm>(EMPTY_FORM);
  const [lastForm, setLastForm] = useState<VendorForm | null>(null);
  if (form !== lastForm) {
    setLastForm(form);
    if (form) setDraft(form);
  }

  const save = useMutation({
    mutationFn: () => {
      const body: VendorInput = {
        name: draft.name.trim(),
        group_name: draft.groupName.trim() || draft.name.trim(),
        models: JSON.stringify(draft.models),
        channel_type: draft.channelType,
      };
      return draft.id === null ? api.createVendor(body) : api.updateVendor(draft.id, body);
    },
    onSuccess: () => {
      toast.success('厂商已保存');
      void qc.invalidateQueries({ queryKey: VENDORS_KEY });
      onClose();
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  return (
    <Dialog open={form !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{draft.id === null ? '新增厂商' : '编辑厂商'}</DialogTitle>
          <DialogDescription>一个模型名只能属于一个厂商，保存时会校验。</DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1"><Label>厂商</Label><Input autoFocus value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} /></div>
            <div className="grid gap-1"><Label>分组名（留空与厂商同名）</Label><Input value={draft.groupName} onChange={(e) => setDraft({ ...draft, groupName: e.target.value })} /></div>
          </div>
          <div className="grid gap-1">
            <Label>渠道类型</Label>
            <Select value={String(draft.channelType)} onValueChange={(v) => setDraft({ ...draft, channelType: v === '24' ? 24 : 58 })}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="58">透传（58）</SelectItem>
                <SelectItem value="24">Gemini 原生（24）</SelectItem>
              </SelectContent>
            </Select>
            <span className="text-xs text-muted-foreground">{TYPE_HELP}</span>
          </div>
          <div className="grid gap-1">
            <Label>包含模型</Label>
            <ChipEditor value={draft.models} onChange={(models) => setDraft({ ...draft, models })} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button disabled={draft.name.trim() === '' || save.isPending} onClick={() => save.mutate()}>保存</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function VendorSection() {
  const qc = useQueryClient();
  const vendors = useQuery({ queryKey: VENDORS_KEY, queryFn: api.vendors });
  const [editing, setEditing] = useState<VendorForm | null>(null);
  const [removing, setRemoving] = useState<Vendor | null>(null);
  const list = [...(vendors.data ?? [])].sort((a, b) => a.order - b.order);

  const refresh = () => void qc.invalidateQueries({ queryKey: VENDORS_KEY });
  const reorder = useMutation({
    mutationFn: (ids: number[]) => api.reorderVendors(ids),
    onSuccess: refresh,
    onError: (e: Error) => toast.error(errorText(e)),
  });
  const remove = useMutation({
    mutationFn: (id: number) => api.deleteVendor(id),
    onSuccess: () => { toast.success('已删除厂商'); refresh(); },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  const move = (index: number, delta: number) => {
    const ids = list.map((v) => v.id);
    const target = index + delta;
    if (target < 0 || target >= ids.length) return;
    [ids[index], ids[target]] = [ids[target], ids[index]];
    reorder.mutate(ids);
  };
  const edit = (v: Vendor) =>
    setEditing({ id: v.id, name: v.name, groupName: v.group_name, channelType: v.channel_type, models: parseStringArray(v.models) });

  return (
    <>
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">厂商决定渠道所在的 NewAPI 分组；顺序即 auto 分组的尝试顺序。</p>
        <Button size="sm" onClick={() => setEditing(EMPTY_FORM)}><Plus className="h-3.5 w-3.5" />新增厂商</Button>
      </div>
      <div className="overflow-x-auto rounded border">
        <table className="w-full text-base">
          <thead className="bg-muted text-left text-sm text-muted-foreground">
            <tr>
              {['厂商', '分组名', '渠道类型', '包含模型', '顺序', ''].map((h) => <th key={h || 'op'} className="h-7 whitespace-nowrap px-2 font-medium">{h}</th>)}
            </tr>
          </thead>
          <tbody>
            {list.map((v, index) => {
              const models = parseStringArray(v.models);
              return (
                <tr key={v.id} className="h-8 border-t">
                  <td className="px-2 font-medium">{v.name}</td>
                  <td className="px-2">{v.group_name}</td>
                  <td className="whitespace-nowrap px-2">{v.channel_type === 24 ? 'Gemini 原生（24）' : '透传（58）'}</td>
                  <td className="max-w-64 truncate px-2" title={models.join('，')}>{models.length} 个：{models.slice(0, 3).join('，')}{models.length > 3 && '…'}</td>
                  <td className="whitespace-nowrap px-2">
                    <Button size="icon" variant="ghost" disabled={index === 0 || reorder.isPending} onClick={() => move(index, -1)} title="上移"><ArrowUp className="h-3.5 w-3.5" /></Button>
                    <Button size="icon" variant="ghost" disabled={index === list.length - 1 || reorder.isPending} onClick={() => move(index, 1)} title="下移"><ArrowDown className="h-3.5 w-3.5" /></Button>
                  </td>
                  <td className="whitespace-nowrap px-2 text-right">
                    <Button size="sm" variant="outline" onClick={() => edit(v)}>编辑</Button>
                    <Button size="icon" variant="ghost" className="text-destructive" onClick={() => setRemoving(v)} title="删除"><Trash2 className="h-3.5 w-3.5" /></Button>
                  </td>
                </tr>
              );
            })}
            {vendors.isLoading && <tr><td colSpan={6} className="h-12 text-center text-muted-foreground">加载中…</td></tr>}
            {vendors.isSuccess && list.length === 0 && <tr><td colSpan={6} className="h-12 text-center text-muted-foreground">还没有厂商</td></tr>}
          </tbody>
        </table>
      </div>
      <p className="text-sm text-muted-foreground">写入 NewAPI 的分组倍率在 M5 实现；nam 不开启默认 auto 分组，用户令牌需选 auto。</p>
      <VendorDialog form={editing} onClose={() => setEditing(null)} />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => { if (!open) setRemoving(null); }}
        title="删除厂商"
        description={`确定删除厂商「${removing?.name ?? ''}」？已使用它的渠道需要自行调整。`}
        confirmLabel="确认删除"
        destructive
        onConfirm={() => {
          if (removing) remove.mutate(removing.id);
          setRemoving(null);
        }}
      />
    </>
  );
}
