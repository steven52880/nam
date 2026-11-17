import { useId, useState } from 'react';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import type { Upstream, UpstreamGroup, UpstreamToken } from '@/api/types';
import { Field } from '@/components/form/Field';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useActionMutation } from '@/hooks/useActionMutation';

export type TokenDialog = 'manual' | 'remote' | null;

interface AddTokenDialogProps {
  upstream: Upstream;
  mode: TokenDialog;
  /** 上游分组：提供后分组输入框带下拉候选（仍可手填）。 */
  groups?: UpstreamGroup[];
  onClose: () => void;
  /** 创建成功后回调，参数为新令牌。 */
  onCreated?: (token: UpstreamToken) => void;
}

export function AddTokenDialog({ upstream, mode, groups, onClose, onCreated }: AddTokenDialogProps) {
  const listId = useId();
  const [name, setName] = useState('');
  const [group, setGroup] = useState('');
  const [key, setKey] = useState('');
  const [quota, setQuota] = useState('');

  const create = useActionMutation({
    mutationFn: () => {
      const quotaUsd = quota.trim() === '' ? null : Number(quota);
      if (mode === 'remote') return api.createRemoteToken(upstream.id, { name: name.trim(), group: group.trim(), quota_usd: quotaUsd });
      return api.createToken(upstream.id, { name: name.trim(), group: group.trim(), key: key.trim() });
    },
    success: mode === 'remote' ? '已在上游新建令牌' : '已录入令牌',
    invalidate: (qc) => invalidate.tokens(qc, upstream.id),
    onSuccess: (token) => {
      setName(''); setGroup(''); setKey(''); setQuota('');
      onClose();
      onCreated?.(token);
    },
  });

  const valid = name.trim() !== '' && (mode === 'remote' || key.trim() !== '') && (quota.trim() === '' || Number(quota) > 0);
  return (
    <Dialog open={mode !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{mode === 'remote' ? '在上游新建令牌' : '手动录入令牌'}</DialogTitle>
          <DialogDescription>{mode === 'remote' ? '会通过上游接口创建一个新令牌。' : '录入已有令牌的 key，只保存在 nam。'}</DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          <Field label="令牌名"><Input autoFocus value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="分组">
            <Input list={groups ? listId : undefined} value={group} onChange={(e) => setGroup(e.target.value)} />
            {groups && (
              <datalist id={listId}>
                {groups.map((g) => <option key={g.name} value={g.name}>{`倍率 ${g.ratio}`}</option>)}
              </datalist>
            )}
          </Field>
          {mode === 'manual' && <Field label="key（sk-…）"><Input type="password" autoComplete="off" value={key} onChange={(e) => setKey(e.target.value)} /></Field>}
          {mode === 'remote' && (
            <Field label="额度（美元）" hint="留空表示不限额度"><Input inputMode="decimal" value={quota} onChange={(e) => setQuota(e.target.value)} /></Field>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button disabled={!valid || create.isPending} onClick={() => create.mutate()}>确定</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
