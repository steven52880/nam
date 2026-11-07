import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import type { Credential, CredType, UpstreamInput, UpstreamType } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input, Textarea } from '@/components/ui/input';
import { Label } from '@/components/ui/misc';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { CRED_OPTIONS, TYPE_OPTIONS } from './constants';

export function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-1">
      <Label>{label}</Label>
      {children}
      {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
    </div>
  );
}

export function CredentialFields({
  credType,
  value,
  onChange,
  userId,
  onUserId,
}: {
  credType: CredType;
  value: Credential;
  onChange: (c: Credential) => void;
  userId: string;
  onUserId: (v: string) => void;
}) {
  if (credType === 'access_token') {
    return (
      <>
        <Field label="Access Token">
          <Input type="password" autoComplete="off" value={value.access_token ?? ''} onChange={(e) => onChange({ ...value, access_token: e.target.value })} />
        </Field>
        <Field label="用户 ID">
          <Input inputMode="numeric" value={userId} onChange={(e) => onUserId(e.target.value)} />
        </Field>
      </>
    );
  }
  if (credType === 'password') {
    return (
      <>
        <Field label="账号">
          <Input autoComplete="off" value={value.username ?? ''} onChange={(e) => onChange({ ...value, username: e.target.value })} />
        </Field>
        <Field label="密码">
          <Input type="password" autoComplete="new-password" value={value.password ?? ''} onChange={(e) => onChange({ ...value, password: e.target.value })} />
        </Field>
      </>
    );
  }
  if (credType === 'sk_key') {
    return (
      <Field label="sk-key" hint="仅 sk-key 没有令牌列表和分组倍率，上游分组折扣需要手填。">
        <Input type="password" autoComplete="off" value={value.key ?? ''} onChange={(e) => onChange({ ...value, key: e.target.value })} />
      </Field>
    );
  }
  return null;
}

export function TypeSelect({ value, onChange }: { value: UpstreamType; onChange: (v: UpstreamType) => void }) {
  return (
    <Select value={value} onValueChange={(v) => onChange(v as UpstreamType)}>
      <SelectTrigger><SelectValue /></SelectTrigger>
      <SelectContent>
        {TYPE_OPTIONS.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
      </SelectContent>
    </Select>
  );
}

export function CreateUpstreamDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const qc = useQueryClient();
  const [name, setName] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [type, setType] = useState<UpstreamType>('newapi');
  const [credType, setCredType] = useState<CredType>('access_token');
  const [cred, setCred] = useState<Credential>({});
  const [userId, setUserId] = useState('');
  const [rate, setRate] = useState('7');
  const [tags, setTags] = useState('');
  const [remark, setRemark] = useState('');

  const urlOk = /^https?:\/\/[^\s/]+/i.test(baseUrl.trim());
  const rateNum = Number(rate);
  const valid = name.trim() !== '' && urlOk && rateNum > 0;

  const create = useMutation({
    mutationFn: () => {
      const body: UpstreamInput = {
        name: name.trim(),
        type,
        base_url: baseUrl.trim(),
        cred_type: credType,
        currency_rate: rateNum,
        tags: tags.trim(),
        remark: remark.trim(),
      };
      if (credType) body.credential = cred;
      if (credType === 'access_token' && userId) body.platform_user_id = Number(userId);
      return api.createUpstream(body);
    },
    onSuccess: (u) => {
      toast.success(`已新增上游「${u.name}」`);
      void qc.invalidateQueries({ queryKey: ['upstreams'] });
      onOpenChange(false);
      setName(''); setBaseUrl(''); setCred({}); setUserId(''); setTags(''); setRemark('');
    },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>新增上游</DialogTitle>
          <DialogDescription>只写入 nam，不会改动 NewAPI。</DialogDescription>
        </DialogHeader>
        <form className="grid gap-3" onSubmit={(e) => { e.preventDefault(); if (valid) create.mutate(); }}>
          <Field label="名称"><Input autoFocus maxLength={64} value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="站点地址" hint={baseUrl && !urlOk ? '地址需以 http:// 或 https:// 开头' : '根地址，不带 /v1'}>
            <Input placeholder="https://example.com" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="类型"><TypeSelect value={type} onChange={setType} /></Field>
            <Field label="凭证方式">
              <Select value={credType === '' ? 'none' : credType} onValueChange={(v) => setCredType(v === 'none' ? '' : (v as CredType))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {CRED_OPTIONS.map(([v, l]) => <SelectItem key={v || 'none'} value={v === '' ? 'none' : v}>{l}</SelectItem>)}
                </SelectContent>
              </Select>
            </Field>
          </div>
          <CredentialFields credType={credType} value={cred} onChange={setCred} userId={userId} onUserId={setUserId} />
          <Field label="上游货币汇率" hint="元 / 上游 1 美元额度。例如填 7，上游余额 10 美元折合 70 元。">
            <Input inputMode="decimal" value={rate} onChange={(e) => setRate(e.target.value)} />
          </Field>
          <Field label="标签" hint="多个标签用英文逗号分隔"><Input value={tags} onChange={(e) => setTags(e.target.value)} /></Field>
          <Field label="备注"><Textarea value={remark} onChange={(e) => setRemark(e.target.value)} /></Field>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
            <Button type="submit" disabled={!valid || create.isPending}>保存</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
