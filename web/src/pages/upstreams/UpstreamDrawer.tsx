import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import type { Upstream, UpstreamInput, UpstreamType } from '@/api/types';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  SheetContent,
} from '@/components/ui/dialog';
import { Input, Textarea } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger, Badge } from '@/components/ui/misc';
import { formatNumber, formatTime } from '@/lib/format';
import { CRED_LABELS } from './constants';
import { errorText } from '@/api/errors';
import { AdapterTab } from './tabs/AdapterTab';
import { CredentialDialog, TestUpstreamDialog } from './tabs/CredentialDialogs';
import { GroupsTab, ModelsTab } from './tabs/GroupsModelsTabs';
import { TokensTab } from './tabs/TokensTab';
import { Field, TypeSelect } from './UpstreamForm';

const PLACEHOLDER_TABS = ['公告', '同步日志'];

function Overview({ u, onClose }: { u: Upstream; onClose: () => void }) {
  const qc = useQueryClient();
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

  const refresh = () => void qc.invalidateQueries({ queryKey: ['upstreams'] });
  const save = useMutation({
    mutationFn: () => {
      const body: UpstreamInput = { name: name.trim(), type, base_url: baseUrl.trim(), tags: tags.trim(), remark: remark.trim(), currency_rate: rateNum };
      return api.updateUpstream(u.id, body);
    },
    onSuccess: () => { toast.success('已保存'); refresh(); },
    onError: (e: Error) => toast.error(errorText(e)),
  });
  const syncNow = useMutation({
    mutationFn: () => api.syncUpstream(u.id, ['tokens', 'groups', 'balance']),
    onSuccess: (results) => {
      const bad = results.filter((r) => !r.ok);
      if (bad.length > 0) toast.error(`同步有失败：${bad.map((r) => r.error ?? r.what).join('；')}`);
      else toast.success('已同步');
      refresh();
      void qc.invalidateQueries({ queryKey: ['upstream-tokens', u.id] });
      void qc.invalidateQueries({ queryKey: ['upstream-groups', u.id] });
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });
  const del = useMutation({
    mutationFn: () => api.deleteUpstream(u.id),
    onSuccess: () => { toast.success(`已删除上游「${u.name}」`); refresh(); onClose(); },
    onError: (e: Error) => toast.error(e.message),
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
        <div className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">最近同步失败：{u.last_sync_error}</div>
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
      <CredentialDialog upstream={u} open={credOpen} onOpenChange={setCredOpen} />
      <TestUpstreamDialog upstream={u} open={testOpen} onOpenChange={setTestOpen} />
      <Dialog open={confirmDel} onOpenChange={setConfirmDel}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>删除上游</DialogTitle>
            <DialogDescription>确定删除「{u.name}」？此操作不可撤销。</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmDel(false)}>取消</Button>
            <Button variant="destructive" disabled={del.isPending} onClick={() => del.mutate()}>确认删除</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
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
  return (
    <Dialog open={upstream !== null} onOpenChange={(v) => { if (!v) onClose(); }}>
      {upstream && (
        <SheetContent>
          <div className="border-b px-4 py-3">
            <DialogTitle>{upstream.name}</DialogTitle>
            <DialogDescription className="truncate">{upstream.base_url}</DialogDescription>
          </div>
          <Tabs defaultValue="overview" className="flex min-h-0 flex-1 flex-col">
            <TabsList className="px-3">
              <TabsTrigger value="overview">概览</TabsTrigger>
              <TabsTrigger value="groups">分组倍率</TabsTrigger>
              <TabsTrigger value="models">上游模型价格</TabsTrigger>
              <TabsTrigger value="tokens">令牌</TabsTrigger>
              <TabsTrigger value="adapter">适配器配置</TabsTrigger>
              {PLACEHOLDER_TABS.map((t) => <TabsTrigger key={t} value={t}>{t}</TabsTrigger>)}
            </TabsList>
            <div className="min-h-0 flex-1 overflow-y-auto">
              <TabsContent value="overview"><Overview key={upstream.id + upstream.updated_at} u={upstream} onClose={onClose} /></TabsContent>
              <TabsContent value="groups"><GroupsTab upstreamId={upstream.id} /></TabsContent>
              <TabsContent value="models"><ModelsTab upstreamId={upstream.id} /></TabsContent>
              <TabsContent value="tokens"><TokensTab upstream={upstream} /></TabsContent>
              <TabsContent value="adapter"><AdapterTab key={upstream.id + upstream.updated_at} upstream={upstream} /></TabsContent>
              {PLACEHOLDER_TABS.map((t) => (
                <TabsContent key={t} value={t}>
                  <div className="m-4 flex h-32 items-center justify-center rounded border border-dashed text-muted-foreground">「{t}」将在后续里程碑实现</div>
                </TabsContent>
              ))}
            </div>
          </Tabs>
        </SheetContent>
      )}
    </Dialog>
  );
}
