import { useMemo } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { useUpstreamGroups, useUpstreamModelPrices } from '@/api/queries';
import type { UpstreamGroup, UpstreamModel } from '@/api/types';
import { groupColumns } from '@/components/entities/groupColumns';
import { DataTable, numberCol, textCol, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import { useActionMutation } from '@/hooks/useActionMutation';
import { formatNumber } from '@/lib/format';
import { DeriveSection } from './DeriveSection';

const getGroupId = (g: UpstreamGroup) => g.name;
const getModelId = (m: UpstreamModel) => String(m.id);

export function GroupsTab({ upstreamId }: { upstreamId: number }) {
  const groups = useUpstreamGroups(upstreamId);
  const sync = useActionMutation({
    mutationFn: () => api.syncUpstream(upstreamId, ['groups']),
    invalidate: (qc) => invalidate.upstream(qc),
    onSuccess: (results) => {
      const bad = results.find((r) => !r.ok);
      if (bad) toast.error(`同步倍率失败：${bad.error ?? ''}`);
      else toast.success('已同步分组倍率');
    },
  });
  const list = useMemo(() => groups.data ?? [], [groups.data]);
  const columns = useMemo(() => groupColumns(), []);
  const status = useMemo(
    () => ({
      loading: groups.isLoading,
      error: groups.isError ? groups.error.message : null,
      empty: '还没有分组倍率，点击「立即同步倍率」',
    }),
    [groups.isLoading, groups.isError, groups.error],
  );

  return (
    <div className="grid gap-2 p-4">
      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">共 {list.length} 个分组</span>
        <Button size="sm" disabled={sync.isPending} onClick={() => sync.mutate()}>{sync.isPending ? '同步中…' : '立即同步倍率'}</Button>
      </div>
      <DataTable<UpstreamGroup>
        layoutId="upstream.groups"
        className="max-h-[60vh]"
        data={list}
        columns={columns}
        getRowId={getGroupId}
        status={status}
      />
    </div>
  );
}

const num = (v: number | null) => (v === null ? '—' : formatNumber(v, 4));

/** 上游模型价格列；列宽单位 rem，列 id 上线后不改。 */
function modelPriceColumns(): TableColumn<UpstreamModel>[] {
  const price = (id: string, header: string, get: (m: UpstreamModel) => number | null): TableColumn<UpstreamModel> =>
    numberCol<UpstreamModel>(id, header, get, { width: 6.5, cell: (m) => num(get(m)) });
  return [
    textCol<UpstreamModel>('model', '模型', (m) => m.model, { width: 14, className: 'font-medium' }),
    textCol<UpstreamModel>('group', '分组', (m) => m.group, { width: 7, cell: (m) => m.group || '—' }),
    price('model_ratio', '倍率', (m) => m.model_ratio),
    price('input', '输入', (m) => m.input),
    price('output', '输出', (m) => m.output),
    price('cache_read', '缓存读', (m) => m.cache_read),
    price('cache_write', '缓存写', (m) => m.cache_write),
    price('per_request', '按次', (m) => m.per_request),
  ];
}

export function ModelsTab({ upstreamId }: { upstreamId: number }) {
  const models = useUpstreamModelPrices(upstreamId);
  const list = useMemo(() => models.data ?? [], [models.data]);
  const columns = useMemo(() => modelPriceColumns(), []);
  const status = useMemo(
    () => ({
      loading: models.isLoading,
      error: models.isError ? models.error.message : null,
      empty: '暂无上游模型价格（价格同步在后续里程碑提供）',
    }),
    [models.isLoading, models.isError, models.error],
  );

  return (
    <div className="grid gap-2 p-4">
      <DeriveSection upstreamId={upstreamId} />
      <span className="text-sm text-muted-foreground">共 {list.length} 条上游模型价格（美元 / 百万，按上游额度计，已换算到各分组）</span>
      <DataTable<UpstreamModel>
        layoutId="upstream.prices"
        className="max-h-[60vh]"
        data={list}
        columns={columns}
        getRowId={getModelId}
        status={status}
      />
    </div>
  );
}
