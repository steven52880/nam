import { Copy } from 'lucide-react';
import { toast } from 'sonner';
import type { Credential, CredType, UpstreamType } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input, Textarea } from '@/components/ui/input';
import { Label } from '@/components/ui/misc';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TYPE_OPTIONS } from './constants';

const USER_ID_SNIPPET = "JSON.parse(localStorage.getItem('user')).id";
const TOKEN_SNIPPET = "Object.keys(localStorage).filter(k=>/token/i.test(k)).forEach(k=>console.log(k, localStorage.getItem(k)))";

function CopySnippet({ code }: { code: string }) {
  const copy = () => {
    navigator.clipboard.writeText(code).then(
      () => toast.success('已复制'),
      () => toast.error('复制失败，请手动选中复制'),
    );
  };
  return (
    <div className="flex items-start gap-1.5">
      <code className="min-w-0 flex-1 break-all rounded bg-muted px-1.5 py-1 font-mono text-xs">{code}</code>
      <Button type="button" size="sm" variant="outline" onClick={copy}><Copy className="h-3 w-3" />复制</Button>
    </div>
  );
}

function HelpBox({ children }: { children: React.ReactNode }) {
  return (
    <details className="rounded border bg-muted/40 px-2 py-1.5 text-sm">
      <summary className="cursor-pointer select-none text-muted-foreground">怎么获取</summary>
      <div className="mt-1.5 grid gap-1.5">{children}</div>
    </details>
  );
}

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
  upstreamType,
}: {
  credType: CredType;
  value: Credential;
  onChange: (c: Credential) => void;
  userId: string;
  onUserId: (v: string) => void;
  upstreamType?: UpstreamType;
}) {
  if (credType === 'cookie') {
    return (
      <>
        <Field label="会话 Cookie（session 的值）">
          <Textarea
            className="font-mono"
            autoComplete="off"
            spellCheck={false}
            placeholder="粘贴 session 的值，也可以粘贴整串 Cookie"
            value={value.cookie ?? ''}
            onChange={(e) => onChange({ ...value, cookie: e.target.value })}
          />
        </Field>
        <Field label={upstreamType === 'onehub' ? '用户 ID（可选）' : '用户 ID'}>
          <Input inputMode="numeric" autoComplete="off" value={userId} onChange={(e) => onUserId(e.target.value)} />
        </Field>
        <HelpBox>
          <div>在浏览器登录上游站，按 F12 → 应用（Application）→ Cookie → 复制 <code className="font-mono">session</code> 的值。</div>
          <div>用户 ID：在控制台（Console）运行下面这行代码。</div>
          <CopySnippet code={USER_ID_SNIPPET} />
          <div className="text-muted-foreground">Cookie 约 30 天过期，在网页上退出登录不会让它失效。</div>
        </HelpBox>
      </>
    );
  }
  if (credType === 'sub2api_session') {
    return (
      <>
        <Field label="access_token">
          <Input type="password" autoComplete="off" spellCheck={false} value={value.access_token ?? ''} onChange={(e) => onChange({ ...value, access_token: e.target.value })} />
        </Field>
        <Field label="refresh_token">
          <Input type="password" autoComplete="off" spellCheck={false} value={value.refresh_token ?? ''} onChange={(e) => onChange({ ...value, refresh_token: e.target.value })} />
        </Field>
        <HelpBox>
          <div>在浏览器登录上游站，按 F12 打开控制台（Console），运行下面这行代码，找到 access_token 和 refresh_token 对应的值。</div>
          <CopySnippet code={TOKEN_SNIPPET} />
          <div className="text-muted-foreground">nam 会自动续期，请不要在别处同时使用这对令牌。</div>
        </HelpBox>
      </>
    );
  }
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
