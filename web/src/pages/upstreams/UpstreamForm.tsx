import { Copy } from 'lucide-react';
import { toast } from 'sonner';
import type { Credential, CredType, UpstreamType } from '@/api/types';
import { Field } from '@/components/form/Field';
import { Button } from '@/components/ui/button';
import { Input, Textarea } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { optionsOf, UPSTREAM_TYPE } from '@/lib/labels';
import { credOptionsWithCurrent, needsUserId } from './constants';

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
    const isRightcode = upstreamType === 'rightcode';
    return (
      <>
        <Field label={isRightcode ? 'user_token' : 'Access Token'}>
          <Input type="password" autoComplete="off" value={value.access_token ?? ''} onChange={(e) => onChange({ ...value, access_token: e.target.value })} />
        </Field>
        {(!upstreamType || needsUserId(upstreamType, 'access_token')) && (
          <Field label="用户 ID">
            <Input inputMode="numeric" value={userId} onChange={(e) => onUserId(e.target.value)} />
          </Field>
        )}
        {isRightcode && (
          <HelpBox>
            <div>在浏览器登录 Right Code，按 F12 → 应用（Application）→ 本地存储（Local Storage）→ 复制 <code className="font-mono">userToken</code> 的值。</div>
            <div className="text-muted-foreground">账号开启了 token 轮换时，到期后需要重新填写。</div>
          </HelpBox>
        )}
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

/** 凭证方式下拉 + 对应输入框。向导和「更换凭证」共用，选项来自 credOptionsWithCurrent。切换方式时调用方负责清空 credential。 */
export function CredentialPicker({
  upstreamType,
  credType,
  onCredType,
  value,
  onChange,
  userId,
  onUserId,
}: {
  upstreamType: UpstreamType;
  credType: CredType;
  onCredType: (t: CredType) => void;
  value: Credential;
  onChange: (c: Credential) => void;
  userId: string;
  onUserId: (v: string) => void;
}) {
  return (
    <>
      <Field label="凭证方式">
        <Select value={credType === '' ? 'none' : credType} onValueChange={(v) => onCredType(v === 'none' ? '' : (v as CredType))}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            {credOptionsWithCurrent(upstreamType, credType).map(([v, l]) => (
              <SelectItem key={v || 'none'} value={v === '' ? 'none' : v}>{l}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <CredentialFields credType={credType} value={value} onChange={onChange} userId={userId} onUserId={onUserId} upstreamType={upstreamType} />
    </>
  );
}

export function TypeSelect({ value, onChange }: { value: UpstreamType; onChange: (v: UpstreamType) => void }) {
  return (
    <Select value={value} onValueChange={(v) => onChange(v as UpstreamType)}>
      <SelectTrigger><SelectValue /></SelectTrigger>
      <SelectContent>
        {optionsOf(UPSTREAM_TYPE).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
      </SelectContent>
    </Select>
  );
}
