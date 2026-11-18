import { useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2 } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { keys } from '@/api/keys';
import { useVendors } from '@/api/queries';
import type { Vendor, VendorInput, VendorUpdateResult } from '@/api/types';
import { ChipEditor } from '@/components/ChipEditor';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { DataTable, customCol, textCol, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/form/Field';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useActionMutation } from '@/hooks/useActionMutation';
import { CHANNEL_TYPE, optionsOf } from '@/lib/labels';
import { parseStringArray } from '@/lib/list';

const TYPE_HELP = '透传（58）适用于 OpenAI / Claude 格式；Gemini 原生格式的渠道必须选 24，否则计费不正确。';

interface VendorForm {
  id: number | null;
  name: string;
  groupName: string;
  channelType: 58 | 24;
  models: string[];
  prefixes: string[];
}

const EMPTY_FORM: VendorForm = { id: null, name: '', groupName: '', channelType: 58, models: [], prefixes: [] };
const PREFIX_HELP = '不区分大小写，最长前缀优先；显式模型优先于前缀。';

function prefixesOf(v: Vendor): string[] {
  return Array.isArray(v.prefixes) ? v.prefixes : [];
}

function VendorDialog({ form, onClose }: { form: VendorForm | null; onClose: () => void }) {
  const [draft, setDraft] = useState<VendorForm>(EMPTY_FORM);
  const [lastForm, setLastForm] = useState<VendorForm | null>(null);
  if (form !== lastForm) {
    setLastForm(form);
    if (form) setDraft(form);
  }

  const save = useActionMutation({
    mutationFn: () => {
      const body: VendorInput = {
        name: draft.name.trim(),
        group_name: draft.groupName.trim() || draft.name.trim(),
        models: draft.models,
        prefixes: draft.prefixes,
        channel_type: draft.channelType,
      };
      return draft.id === null ? api.createVendor(body) : api.updateVendor(draft.id, body);
    },
    success: (res) => {
      const drafts = draft.id === null ? 0 : (res as VendorUpdateResult).group_drafts;
      return drafts > 0 ? `已为 ${drafts} 个渠道生成分组草稿，请到渠道页应用变更` : '厂商已保存';
    },
    invalidate: invalidate.vendors,
    onSuccess: onClose,
  });

  return (
    <Dialog open={form !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent size="xl">
        <DialogHeader>
          <DialogTitle>{draft.id === null ? '新增厂商' : '编辑厂商'}</DialogTitle>
          <DialogDescription>一个模型名只能属于一个厂商，保存时会校验。</DialogDescription>
        </DialogHeader>
        <DialogBody className="grid content-start gap-3">
          <div className="grid grid-cols-2 gap-3">
            <Field label="厂商"><Input autoFocus value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} /></Field>
            <Field label="分组名（留空与厂商同名）"><Input value={draft.groupName} onChange={(e) => setDraft({ ...draft, groupName: e.target.value })} /></Field>
          </div>
          <Field label="渠道类型" hint={TYPE_HELP}>
            <Select value={String(draft.channelType)} onValueChange={(v) => setDraft({ ...draft, channelType: v === '24' ? 24 : 58 })}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {optionsOf(CHANNEL_TYPE, [58, 24]).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
              </SelectContent>
            </Select>
          </Field>
          <Field label="包含模型">
            <ChipEditor value={draft.models} onChange={(models) => setDraft({ ...draft, models })} />
          </Field>
          <Field label="模型名前缀" hint={PREFIX_HELP}>
            <ChipEditor
              value={draft.prefixes}
              placeholder="如 gpt-、claude-，回车添加"
              onChange={(prefixes) => setDraft({ ...draft, prefixes })}
            />
          </Field>
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button disabled={draft.name.trim() === '' || save.isPending} onClick={() => save.mutate()}>保存</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const vendorRowId = (v: Vendor) => String(v.id);
const VENDOR_STATUS_LOADING = { loading: true, empty: '还没有厂商' };
const VENDOR_STATUS_IDLE = { empty: '还没有厂商' };

interface ReorderVars {
  ids: number[];
  /** 拖动前的完整厂商列表，失败时恢复。 */
  snapshot: Vendor[];
}

export function VendorSection() {
  const qc = useQueryClient();
  const vendors = useVendors();
  const [editing, setEditing] = useState<VendorForm | null>(null);
  const [removing, setRemoving] = useState<Vendor | null>(null);
  const list = useMemo(() => [...(vendors.data ?? [])].sort((a, b) => a.order - b.order), [vendors.data]);

  const reorder = useActionMutation<ReorderVars>({
    mutationFn: ({ ids }) => api.reorderVendors(ids),
    // 失败：恢复拖动前的完整快照；错误提示走默认的 toast。
    onError: (_err, vars) => {
      qc.setQueryData(keys.vendors.list(), vars.snapshot);
    },
    // 无论成败都重新取一次。
    onSettled: () => {
      void invalidate.vendors(qc);
    },
  });
  const remove = useActionMutation({
    mutationFn: (id: number) => api.deleteVendor(id),
    success: '已删除厂商',
    invalidate: invalidate.vendors,
  });

  // 拖动结束：先停掉进行中的查询，保存快照，乐观写入新顺序，再提交。
  const onRowReorder = useCallback(
    async (rowIds: string[]) => {
      await qc.cancelQueries({ queryKey: keys.vendors.all() });
      const snapshot = qc.getQueryData<Vendor[]>(keys.vendors.list());
      if (!snapshot) return;
      const byId = new Map(snapshot.map((v) => [String(v.id), v]));
      const next: Vendor[] = [];
      for (const rid of rowIds) {
        const v = byId.get(rid);
        if (v) next.push({ ...v, order: next.length });
      }
      if (next.length !== snapshot.length) return;
      qc.setQueryData(keys.vendors.list(), next);
      reorder.mutate({ ids: next.map((v) => v.id), snapshot });
    },
    [qc, reorder.mutate],
  );

  const edit = useCallback(
    (v: Vendor) =>
      setEditing({ id: v.id, name: v.name, groupName: v.group_name, channelType: v.channel_type, models: parseStringArray(v.models), prefixes: prefixesOf(v) }),
    [],
  );

  const columns = useMemo<TableColumn<Vendor>[]>(
    () => [
      textCol<Vendor>('name', '厂商', (v) => v.name, { width: 9, cell: (v) => <span className="font-medium">{v.name}</span> }),
      textCol<Vendor>('group_name', '分组名', (v) => v.group_name, { width: 9 }),
      textCol<Vendor>('channel_type', '渠道类型', (v) => CHANNEL_TYPE[v.channel_type === 24 ? 24 : 58].label, { width: 8 }),
      customCol<Vendor>('models', '包含模型', {
        width: 16,
        accessor: (v) => parseStringArray(v.models).length,
        sortType: 'number',
        cell: (v) => {
          const models = parseStringArray(v.models);
          return <span title={models.join('，')}><ModelCountCell models={models} variant="preview" /></span>;
        },
      }),
      textCol<Vendor>('prefixes', '模型名前缀', (v) => prefixesOf(v).join('，'), {
        width: 12,
        cell: (v) => {
          const prefixes = prefixesOf(v);
          return <span title={prefixes.join('，')}>{prefixes.length > 0 ? prefixes.join('，') : '—'}</span>;
        },
      }),
      customCol<Vendor>('actions', '', {
        width: 8,
        fixed: true,
        align: 'right',
        cell: (v) => (
          <>
            <Button size="sm" variant="outline" onClick={() => edit(v)}>编辑</Button>
            <Button size="icon" variant="ghost" className="text-destructive" onClick={() => setRemoving(v)} title="删除"><Trash2 className="h-3.5 w-3.5" /></Button>
          </>
        ),
      }),
    ],
    [edit],
  );

  return (
    <>
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">厂商决定渠道所在的 NewAPI 分组；顺序即 auto 分组的尝试顺序。</p>
        <Button size="sm" onClick={() => setEditing(EMPTY_FORM)}><Plus className="h-3.5 w-3.5" />新增厂商</Button>
      </div>
      <DataTable<Vendor>
        layoutId="settings.vendors"
        data={list}
        columns={columns}
        getRowId={vendorRowId}
        status={vendors.isLoading ? VENDOR_STATUS_LOADING : VENDOR_STATUS_IDLE}
        onRowReorder={onRowReorder}
        rowReorderDisabled={reorder.isPending}
      />
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
