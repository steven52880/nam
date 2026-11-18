import { Plus } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { invalidate } from '@/api/invalidate';
import { useUpstreams } from '@/api/queries';
import type { Upstream } from '@/api/types';
import { FilterBar } from '@/components/filters/FilterBar';
import { ALL_FILTER, FilterSelect } from '@/components/filters/FilterSelect';
import { SearchInput } from '@/components/filters/SearchInput';
import { PageHeader } from '@/components/layout/PageHeader';
import { Button } from '@/components/ui/button';
import { DataTable } from '@/components/table';
import { useActionMutation } from '@/hooks/useActionMutation';
import { optionsOf, UPSTREAM_TYPE } from '@/lib/labels';
import { useUrlState } from '@/lib/url-state';
import { allTags, splitTags } from './upstreams/constants';
import { CredentialDialog } from './upstreams/tabs/CredentialDialogs';
import { UpstreamDrawer } from './upstreams/UpstreamDrawer';
import { buildUpstreamPageColumns } from './upstreams/upstreamPageColumns';
import { AddUpstreamWizard } from './upstreams/wizard/AddUpstreamWizard';

const ALL = ALL_FILTER;
const SYNC_ALL = ['tokens', 'groups', 'balance'];
const NO_IDS: ReadonlySet<number> = new Set();
const getUpstreamId = (u: Upstream) => String(u.id);

export default function UpstreamsPage() {
  const urlState = useUrlState();
  const [type, setType] = useState(ALL);
  const [tag, setTag] = useState(ALL);
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);
  const [credTarget, setCredTarget] = useState<Upstream | null>(null);
  const [checked, setChecked] = useState<Set<string>>(new Set());
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
  const syncMutate = sync.mutate;
  const syncingIds = useMemo<ReadonlySet<number>>(
    () => (sync.isPending && sync.variables ? new Set(sync.variables) : NO_IDS),
    [sync.isPending, sync.variables],
  );
  const syncOne = useCallback((id: number) => syncMutate([id]), [syncMutate]);

  const list = useMemo(() => query.data ?? [], [query.data]);
  const openId = Number(urlState.get('open')) || null;
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

  // 原来就是 replace，不进浏览器历史，保持不变
  const patchUrl = urlState.patch;
  const setOpen = useCallback((id: number | null) => patchUrl({ open: id === null ? null : String(id) }), [patchUrl]);
  const openRow = useCallback((u: Upstream) => setOpen(u.id), [setOpen]);
  const filtering = type !== ALL || tag !== ALL || search !== '';

  const columns = useMemo(
    () => buildUpstreamPageColumns({ syncingIds, onSync: syncOne, onReplaceCred: setCredTarget }),
    [syncingIds, syncOne],
  );
  const emptyText = useMemo(
    () => (
      <>
        {filtering ? '没有符合条件的上游' : '还没有上游，点击「新增上游」开始'}
        {filtering && (
          <Button variant="link" onClick={() => { setType(ALL); setTag(ALL); setSearch(''); }}>清除筛选</Button>
        )}
      </>
    ),
    [filtering],
  );
  const status = useMemo(
    () => ({ loading: query.isLoading, error: query.isError ? query.error.message : null, empty: emptyText }),
    [query.isLoading, query.isError, query.error, emptyText],
  );
  const rowSelection = useMemo(
    () => ({ mode: 'multi' as const, selected: checked, onChange: setChecked }),
    [checked],
  );

  return (
    <div className="flex flex-col gap-3 p-4">
      <PageHeader title="上游" description="管理上游站点、凭证和上游货币汇率。" />
      <FilterBar clearable={false}>
        <Button onClick={() => setCreating(true)}><Plus className="h-3.5 w-3.5" />新增上游</Button>
        <Button variant="outline" disabled={checked.size === 0 || sync.isPending} onClick={() => sync.mutate([...checked].map(Number))}>
          批量同步{checked.size > 0 && `（${checked.size}）`}
        </Button>
        <FilterSelect label="类型" value={type} options={optionsOf(UPSTREAM_TYPE)} onChange={setType} />
        <FilterSelect label="标签" value={tag} options={tags.map((t): [string, string] => [t, t])} onChange={setTag} />
        <SearchInput value={search} onChange={setSearch} placeholder="搜索名称或地址" />
      </FilterBar>
      <DataTable<Upstream>
        layoutId="upstreams.list"
        className="max-h-[70vh]"
        data={filtered}
        columns={columns}
        getRowId={getUpstreamId}
        status={status}
        rowSelection={rowSelection}
        onRowClick={openRow}
      />
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
