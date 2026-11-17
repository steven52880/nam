import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import type { Upstream, UpstreamInput, UpstreamType } from '@/api/types';
import { Field } from '@/components/form/Field';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Banner } from '@/components/ui/banner';
import { Button } from '@/components/ui/button';
import { DrawerShell } from '@/components/ui/dialog';
import { Input, Textarea } from '@/components/ui/input';
import { Badge } from '@/components/ui/misc';
import { formatNumber, formatTime } from '@/lib/format';
import { CRED_LABELS } from './constants';
import { CredNote, isCredInvalid } from './credStatus';
import { invalidate } from '@/api/invalidate';
import { useActionMutation } from '@/hooks/useActionMutation';
import { AdapterTab } from './tabs/AdapterTab';
import { CredentialDialog, TestUpstreamDialog } from './tabs/CredentialDialogs';
import { GroupsTab, ModelsTab } from './tabs/GroupsModelsTabs';
import { TokensTab } from './tabs/TokensTab';
import { TypeSelect } from './UpstreamForm';

const PLACEHOLDER_TABS = ['公告', '同步日志'];

function Overview({ u, onClose }: { u: Upstream; onClose: () => void }) {
  const [name, setName] = useState(u.name);
  const [baseUrl, setBaseUrl] = useState(u.base_url);
  const [type, setType] = useState<UpstreamType>(u.type);
  const [tags, setTags] = useState(u.tags);
  const [remark, setRemark] = useState(u.remark);
  const [rate, setRate] = useState(String(u.currency_rate));
  const [confirmDel, setConfirmDel] = useState(false);
  const [credOpen, setCredOpen] = useState(false);
  const [testOpen, setTestOpen] = useState(false);

  const rateNum = Number(rate);
  const valid = name.trim() !== '' && /^https?:\/\//i.test(baseUrl.trim()) && rateNum > 0;

  const save = useActionMutation({
    mutationFn: () => {
      const body: UpstreamInput = { name: name.trim(), type, base_url: baseUrl.trim(), tags: tags.trim(), remark: remark.trim(), currency_rate: rateNum };
      return api.updateUpstream(u.id, body);
    },
    success: '已保存',
    invalidate: (qc) => invalidate.upstream(qc),
  });
  const syncNow = useActionMutation({
    mutationFn: () => api.syncUpstream(u.id, ['tokens', 'groups', 'balance']),
    invalidate: (qc) => invalidate.upstream(qc),
    onSuccess: (results) => {
      const bad = results.filter((r) => !r.ok);
      if (bad.length > 0) toast.error(`同步有失败：${bad.map((r) => r.error ?? r.what).join('；')}`);
      else toast.success('已同步');
    },
  });
  const del = useActionMutation({
    mutationFn: () => api.deleteUpstream(u.id),
    success: `已删除上游「${u.name}」`,
    invalidate: (qc) => invalidate.upstream(qc),
    onSuccess: () => onClose(),
  });

  return (
    <div className="grid gap-3 p-4">
      <div className="grid grid-cols-4 gap-2 text-center">
        <Stat label="余额（美元）" value={formatNumber(u.balance_usd)} />
        <Stat label="折合人民币" value={formatNumber(u.balance_cny)} />
        <Stat label="渠道数 / 令牌数" value={`${u.channel_count} / ${u.token_count}`} />
        <Stat label="最近同步" value={formatTime(u.last_sync_at)} small />
      </div>
      {u.last_sync_error && (
        <Banner
          tone="error"
          action={isCredInvalid(u) ? <Button size="sm" variant="destructive" onClick={() => setCredOpen(true)}>更换凭证</Button> : undefined}
        >
          最近同步失败：{u.last_sync_error}
        </Banner>
      )}
      <Field label="名称"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
      <Field label="站点地址"><Input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} /></Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="类型"><TypeSelect value={type} onChange={setType} /></Field>
        <Field label="上游货币汇率" hint="修改后会影响该上游所有渠道的上游渠道成本价。">
          <Input inputMode="decimal" value={rate} onChange={(e) => setRate(e.target.value)} />
        </Field>
      </div>
      <Field label="标签" hint="多个标签用英文逗号分隔"><Input value={tags} onChange={(e) => setTags(e.target.value)} /></Field>
      <Field label="备注"><Textarea value={remark} onChange={(e) => setRemark(e.target.value)} /></Field>
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">凭证状态：</span>
        {u.has_credential ? <Badge tone="success">已配置（{CRED_LABELS[u.cred_type] ?? u.cred_type}）</Badge> : <Badge tone="warning">未配置</Badge>}
        <CredNote u={u} />
        <Button size="sm" variant="outline" onClick={() => setCredOpen(true)}>更换凭证</Button>
        <Button size="sm" variant="outline" onClick={() => setTestOpen(true)}>测试连通性</Button>
      </div>
      <div className="flex justify-between border-t pt-3">
        <Button variant="destructive" onClick={() => setConfirmDel(true)}>删除上游</Button>
        <div className="flex gap-2">
          <Button variant="outline" disabled={syncNow.isPending} onClick={() => syncNow.mutate()}>
            {syncNow.isPending ? '同步中…' : '立即同步'}
          </Button>
          <Button disabled={!valid || save.isPending} onClick={() => save.mutate()}>保存</Button>
        </div>
      </div>
      <CredentialDialog upstream={u} type={type} open={credOpen} onOpenChange={setCredOpen} />
      <TestUpstreamDialog upstream={u} open={testOpen} onOpenChange={setTestOpen} />
      <ConfirmDialog
        open={confirmDel}
        onOpenChange={setConfirmDel}
        title="删除上游"
        description={`确定删除「${u.name}」？此操作不可撤销。`}
        confirmLabel="确认删除"
        destructive
        pending={del.isPending}
        onConfirm={() => del.mutate()}
      />
    </div>
  );
}

function Stat({ label, value, small }: { label: string; value: string; small?: boolean }) {
  return (
    <div className="rounded border bg-muted/50 px-2 py-1.5">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={small ? 'text-sm tabular-nums' : 'text-lg font-semibold tabular-nums'}>{value}</div>
    </div>
  );
}

export function UpstreamDrawer({ upstream, onClose }: { upstream: Upstream | null; onClose: () => void }) {
  if (!upstream) return null;
  return (
    <DrawerShell
      open
      onOpenChange={(v) => { if (!v) onClose(); }}
      title={upstream.name}
      description={upstream.base_url}
      tabs={[
        { value: 'overview', label: '概览', content: <Overview key={upstream.id} u={upstream} onClose={onClose} /> },
        { value: 'groups', label: '分组倍率', content: <GroupsTab upstreamId={upstream.id} /> },
        { value: 'models', label: '上游模型价格', content: <ModelsTab upstreamId={upstream.id} /> },
        { value: 'tokens', label: '令牌', content: <TokensTab upstream={upstream} /> },
        { value: 'adapter', label: '适配器配置', content: <AdapterTab key={upstream.id + upstream.updated_at} upstream={upstream} /> },
        ...PLACEHOLDER_TABS.map((t) => ({
          value: t,
          label: t,
          content: (
            <div className="m-4 flex h-32 items-center justify-center rounded border border-dashed text-muted-foreground">「{t}」将在后续里程碑实现</div>
          ),
        })),
      ]}
      defaultTab="overview"
    />
  );
}
