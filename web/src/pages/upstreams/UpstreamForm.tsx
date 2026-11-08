import type { Credential, CredType, UpstreamType } from '@/api/types';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/misc';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TYPE_OPTIONS } from './constants';

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
