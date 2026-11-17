import { Plus } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { invalidate } from '@/api/invalidate';
import { useUpstreams } from '@/api/queries';
import type { Upstream } from '@/api/types';
import { PageHeader } from '@/components/layout/PageHeader';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/misc';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useActionMutation } from '@/hooks/useActionMutation';
import { formatNumber, formatTime } from '@/lib/format';
import { optionsOf, UPSTREAM_TYPE } from '@/lib/labels';
import { allTags, CRED_LABELS, splitTags } from './upstreams/constants';
import { CredNote, isCredInvalid } from './upstreams/credStatus';
import { CredentialDialog } from './upstreams/tabs/CredentialDialogs';
import { UpstreamDrawer } from './upstreams/UpstreamDrawer';
import { AddUpstreamWizard } from './upstreams/wizard/AddUpstreamWizard';

const ALL = 'all';
const HEADERS = ['', '名称和地址', '类型', '标签', '上游货币汇率', '凭证状态', '余额（美元）', '折合人民币', '令牌数', '渠道数', '最近同步', '错误', ''];
const RIGHT_ALIGNED = new Set([4, 6, 7, 8, 9]);
const SYNC_ALL = ['tokens', 'groups', 'balance'];

interface RowProps {
  u: Upstream;
  checked: boolean;
  syncing: boolean;
  onCheck: () => void;
  onOpen: () => void;
  onSync: () => void;
  onReplaceCred: () => void;
}

function Row({ u, checked, syncing, onCheck, onOpen, onSync, onReplaceCred }: RowProps) {
  return (
    <tr className="cursor-pointer border-t hover:bg-accent/40" onClick={onOpen}>
      <td className="px-3" onClick={(e) => e.stopPropagation()}>
        <input type="checkbox" checked={checked} onChange={onCheck} />
      </td>
      <td className="h-8 px-3">
        <div className="font-medium leading-4">{u.name}</div>
        <div className="max-w-56 truncate text-xs leading-4 text-muted-foreground">{u.base_url}</div>
      </td>
      <td className="px-3"><Badge>{u.type}</Badge></td>
      <td className="px-3"><div className="flex gap-1">{splitTags(u.tags).map((t) => <Badge key={t} tone="accent">{t}</Badge>)}</div></td>
      <td className="px-3 text-right tabular-nums">{formatNumber(u.currency_rate)}</td>
      <td className="px-3">
        <div className="flex flex-col items-start gap-0.5">
          {u.has_credential ? <Badge tone="success">{CRED_LABELS[u.cred_type] ?? '已配置'}</Badge> : <Badge tone="warning">未配置</Badge>}
          <CredNote u={u} />
        </div>
      </td>
      <td className="px-3 text-right tabular-nums">{formatNumber(u.balance_usd)}</td>
      <td className="px-3 text-right tabular-nums">{formatNumber(u.balance_cny)}</td>
      <td className="px-3 text-right tabular-nums">{u.token_count}</td>
      <td className="px-3 text-right tabular-nums">{u.channel_count}</td>
      <td className="whitespace-nowrap px-3 tabular-nums">{formatTime(u.last_sync_at)}</td>
      <td className="px-3 text-destructive" title={u.last_sync_error} onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center gap-1.5">
          <span className="max-w-48 truncate">{u.last_sync_error}</span>
          {isCredInvalid(u) && <Button size="sm" variant="outline" onClick={onReplaceCred}>更换凭证</Button>}
        </div>
      </td>
      <td className="px-3" onClick={(e) => e.stopPropagation()}>
        <Button size="sm" variant="outline" disabled={syncing} onClick={onSync}>{syncing ? '同步中…' : '立即同步'}</Button>
      </td>
    </tr>
  );
}

export default function UpstreamsPage() {
  const [params, setParams] = useSearchParams();
  const [type, setType] = useState(ALL);
  const [tag, setTag] = useState(ALL);
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);
  const [credTarget, setCredTarget] = useState<Upstream | null>(null);
  const [checked, setChecked] = useState<Set<number>>(new Set());
  const query = useUpstreams();

  const sync = useActionMutation({
    mutationFn: async (ids: number[]) => {
      const failures: string[] = [];
      for (const id of ids) {
        try {
          const results = await api.syncUpstream(id, SYNC_ALL);
          for (const r of results) if (!r.ok) failures.push(`${r.what}: ${r.error ?? '失败'}`);
        } catch (e) {
          failures.push(errorText(e));
        }
      }
      return failures;
    },
    invalidate: (qc) => invalidate.upstream(qc),
    onSuccess: (failures) => {
      if (failures.length > 0) toast.error(`同步有失败：${failures.join('；')}`);
      else toast.success('同步完成');
    },
  });
  const toggleChecked = (id: number) =>
    setChecked((cur) => {
      const next = new Set(cur);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const list = useMemo(() => query.data ?? [], [query.data]);
  const openId = Number(params.get('open')) || null;
  const opened = list.find((u) => u.id === openId) ?? null;
  const tags = useMemo(() => allTags(list), [list]);

  const filtered = useMemo(() => {
    const s = search.trim().toLowerCase();
    return list.filter(
      (u) =>
        (type === ALL || u.type === type) &&
        (tag === ALL || splitTags(u.tags).includes(tag)) &&
        (!s || u.name.toLowerCase().includes(s) || u.base_url.toLowerCase().includes(s)),
    );
  }, [list, type, tag, search]);

  const setOpen = (id: number | null) => {
    const next = new URLSearchParams(params);
    if (id === null) next.delete('open');
    else next.set('open', String(id));
    setParams(next, { replace: true });
  };
  const filtering = type !== ALL || tag !== ALL || search !== '';

  return (
    <div className="flex flex-col gap-3 p-4">
      <PageHeader title="上游" description="管理上游站点、凭证和上游货币汇率。" />
      <div className="flex flex-wrap items-center gap-2">
        <Button onClick={() => setCreating(true)}><Plus className="h-3.5 w-3.5" />新增上游</Button>
        <Button variant="outline" disabled={checked.size === 0 || sync.isPending} onClick={() => sync.mutate([...checked])}>
          批量同步{checked.size > 0 && `（${checked.size}）`}
        </Button>
        <Select value={type} onValueChange={setType}>
          <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>全部类型</SelectItem>
            {optionsOf(UPSTREAM_TYPE).map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
          </SelectContent>
        </Select>
        <Select value={tag} onValueChange={setTag}>
          <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>全部标签</SelectItem>
            {tags.map((t) => <SelectItem key={t} value={t}>{t}</SelectItem>)}
          </SelectContent>
        </Select>
        <Input className="w-56" placeholder="搜索名称或地址" value={search} onChange={(e) => setSearch(e.target.value)} />
      </div>
      <div className="overflow-x-auto rounded border bg-card">
        <table className="w-full text-base">
          <thead className="bg-muted text-left text-sm text-muted-foreground">
            <tr>
              {HEADERS.map((h, i) => (
                <th key={`${h}-${i}`} className={`h-7 whitespace-nowrap px-3 font-medium ${RIGHT_ALIGNED.has(i) ? 'text-right' : ''}`}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((u) => (
              <Row
                key={u.id}
                u={u}
                checked={checked.has(u.id)}
                syncing={sync.isPending && sync.variables?.includes(u.id) === true}
                onCheck={() => toggleChecked(u.id)}
                onOpen={() => setOpen(u.id)}
                onSync={() => sync.mutate([u.id])}
                onReplaceCred={() => setCredTarget(u)}
              />
            ))}
            {query.isLoading && <tr><td colSpan={HEADERS.length} className="h-16 text-center text-muted-foreground">加载中…</td></tr>}
            {query.isError && <tr><td colSpan={HEADERS.length} className="h-16 text-center text-destructive">{query.error.message}</td></tr>}
            {query.isSuccess && filtered.length === 0 && (
              <tr>
                <td colSpan={HEADERS.length} className="h-24 text-center text-muted-foreground">
                  {filtering ? '没有符合条件的上游' : '还没有上游，点击「新增上游」开始'}
                  {filtering && (
                    <Button variant="link" onClick={() => { setType(ALL); setTag(ALL); setSearch(''); }}>清除筛选</Button>
                  )}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <AddUpstreamWizard open={creating} onOpenChange={setCreating} />
      <UpstreamDrawer upstream={opened} onClose={() => setOpen(null)} />
      {credTarget && (
        <CredentialDialog
          key={credTarget.id}
          upstream={credTarget}
          open
          onOpenChange={(o) => { if (!o) setCredTarget(null); }}
        />
      )}
    </div>
  );
}
