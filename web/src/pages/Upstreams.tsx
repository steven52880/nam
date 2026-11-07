import { useQuery } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { api } from '@/api/endpoints';
import type { Upstream } from '@/api/types';
import { PageHeader } from '@/components/layout/PageHeader';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/misc';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { formatNumber, formatTime } from '@/lib/format';
import { allTags, CRED_LABELS, splitTags, TYPE_OPTIONS } from './upstreams/constants';
import { UpstreamDrawer } from './upstreams/UpstreamDrawer';
import { CreateUpstreamDialog } from './upstreams/UpstreamForm';

const ALL = 'all';
const HEADERS = ['名称和地址', '类型', '标签', '上游货币汇率', '凭证状态', '余额（美元）', '折合人民币', '令牌数', '渠道数', '最近同步', '错误'];
const RIGHT_ALIGNED = new Set([3, 5, 6, 7, 8]);

function Row({ u, onOpen }: { u: Upstream; onOpen: () => void }) {
  return (
    <tr className="cursor-pointer border-t hover:bg-accent/40" onClick={onOpen}>
      <td className="h-8 px-3">
        <div className="font-medium leading-4">{u.name}</div>
        <div className="max-w-56 truncate text-xs leading-4 text-muted-foreground">{u.base_url}</div>
      </td>
      <td className="px-3"><Badge>{u.type}</Badge></td>
      <td className="px-3"><div className="flex gap-1">{splitTags(u.tags).map((t) => <Badge key={t} tone="accent">{t}</Badge>)}</div></td>
      <td className="px-3 text-right tabular-nums">{formatNumber(u.currency_rate)}</td>
      <td className="px-3">
        {u.has_credential ? <Badge tone="success">{CRED_LABELS[u.cred_type] ?? '已配置'}</Badge> : <Badge tone="warning">未配置</Badge>}
      </td>
      <td className="px-3 text-right tabular-nums">{formatNumber(u.balance_usd)}</td>
      <td className="px-3 text-right tabular-nums">{formatNumber(u.balance_cny)}</td>
      <td className="px-3 text-right tabular-nums">{u.token_count}</td>
      <td className="px-3 text-right tabular-nums">{u.channel_count}</td>
      <td className="whitespace-nowrap px-3 tabular-nums">{formatTime(u.last_sync_at)}</td>
      <td className="max-w-48 truncate px-3 text-destructive" title={u.last_sync_error}>{u.last_sync_error}</td>
    </tr>
  );
}

export default function UpstreamsPage() {
  const [params, setParams] = useSearchParams();
  const [type, setType] = useState(ALL);
  const [tag, setTag] = useState(ALL);
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);
  const query = useQuery({ queryKey: ['upstreams'], queryFn: api.upstreams });

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
        <Select value={type} onValueChange={setType}>
          <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>全部类型</SelectItem>
            {TYPE_OPTIONS.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
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
                <th key={h} className={`h-7 whitespace-nowrap px-3 font-medium ${RIGHT_ALIGNED.has(i) ? 'text-right' : ''}`}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((u) => <Row key={u.id} u={u} onOpen={() => setOpen(u.id)} />)}
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
      <CreateUpstreamDialog open={creating} onOpenChange={setCreating} />
      <UpstreamDrawer upstream={opened} onClose={() => setOpen(null)} />
    </div>
  );
}
