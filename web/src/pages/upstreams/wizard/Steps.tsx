import { useMutation } from '@tanstack/react-query';
import { useEffect } from 'react';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { CredType, UpstreamType } from '@/api/types';
import { Input, Textarea } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { CRED_OPTIONS } from '../constants';
import { CredentialFields, Field, TypeSelect } from '../UpstreamForm';
import type { UpstreamDraft } from './state';

interface StepProps {
  draft: UpstreamDraft;
  onChange: (patch: Partial<UpstreamDraft>) => void;
}

export function StepBasics({ draft, onChange }: StepProps) {
  const urlOk = /^https?:\/\/[^\s/]+/i.test(draft.baseUrl.trim());
  return (
    <div className="grid gap-3">
      <Field label="名称">
        <Input autoFocus maxLength={64} value={draft.name} onChange={(e) => onChange({ name: e.target.value })} />
      </Field>
      <Field label="站点地址" hint={draft.baseUrl && !urlOk ? '地址需以 http:// 或 https:// 开头' : '根地址，不带 /v1'}>
        <Input placeholder="https://example.com" value={draft.baseUrl} onChange={(e) => onChange({ baseUrl: e.target.value })} />
      </Field>
    </div>
  );
}

export function StepDetect({ draft, onChange }: StepProps) {
  const probe = useMutation({
    mutationFn: () => api.probeUpstream(draft.baseUrl.trim()),
    onSuccess: (res) => onChange({ type: res.type }),
  });

  useEffect(() => {
    probe.mutate();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="grid gap-3">
      <div className="rounded border bg-muted/40 px-3 py-2">
        {probe.isPending && <span className="text-muted-foreground">正在识别站点类型…</span>}
        {probe.isSuccess && (
          <div className="grid gap-0.5">
            <div>识别结果：<span className="font-medium">{probe.data.type}</span>{probe.data.version && <span className="text-muted-foreground">（版本 {probe.data.version}）</span>}</div>
            <div className="text-sm text-muted-foreground">依据：{probe.data.evidence || '无'}</div>
          </div>
        )}
        {probe.isError && <span className="text-sm text-destructive">自动识别失败：{errorText(probe.error)}。请手动选择类型。</span>}
      </div>
      <Field label="上游类型（可手动修改）">
        <TypeSelect value={draft.type} onChange={(type: UpstreamType) => onChange({ type })} />
      </Field>
    </div>
  );
}

export function StepCredential({ draft, onChange }: StepProps) {
  return (
    <div className="grid gap-3">
      <Field label="凭证方式">
        <Select
          value={draft.credType === '' ? 'none' : draft.credType}
          onValueChange={(v) => onChange({ credType: v === 'none' ? '' : (v as CredType), credential: {} })}
        >
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            {CRED_OPTIONS.map(([v, l]) => <SelectItem key={v || 'none'} value={v === '' ? 'none' : v}>{l}</SelectItem>)}
          </SelectContent>
        </Select>
      </Field>
      <CredentialFields
        credType={draft.credType}
        value={draft.credential}
        onChange={(credential) => onChange({ credential })}
        userId={draft.userId}
        onUserId={(userId) => onChange({ userId })}
      />
    </div>
  );
}

export function StepFinish({ draft, onChange }: StepProps) {
  const rate = Number(draft.rate);
  const example = rate > 0 ? `上游 1 美元额度 = ${rate} 元` : '上游 1 美元额度 = X 元';
  return (
    <div className="grid gap-3">
      <Field label="上游货币汇率" hint={`元 / 上游 1 美元额度。例如：${example}，上游余额 10 美元折合 ${rate > 0 ? rate * 10 : 'X × 10'} 元。`}>
        <Input inputMode="decimal" value={draft.rate} onChange={(e) => onChange({ rate: e.target.value })} />
      </Field>
      <Field label="标签" hint="多个标签用英文逗号分隔">
        <Input value={draft.tags} onChange={(e) => onChange({ tags: e.target.value })} />
      </Field>
      <Field label="备注">
        <Textarea value={draft.remark} onChange={(e) => onChange({ remark: e.target.value })} />
      </Field>
      <label className="flex items-center gap-2 text-base">
        <input type="checkbox" checked={draft.syncNow} onChange={(e) => onChange({ syncNow: e.target.checked })} />
        保存后立即同步令牌、分组倍率和余额
      </label>
    </div>
  );
}
