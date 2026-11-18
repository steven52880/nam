import { useCallback, useMemo } from 'react';
import { useUpstreams } from '@/api/queries';
import type { Upstream } from '@/api/types';
import { upstreamColumns } from '@/components/entities/upstreamColumns';
import { DataTable, pickColumns } from '@/components/table';

const getUpstreamId = (u: Upstream) => String(u.id);
const COLUMN_IDS = ['name', 'type', 'currency_rate', 'token_count', 'channel_count'];

export function StepUpstream({ value, onChange }: { value: number | null; onChange: (id: number) => void }) {
  const upstreams = useUpstreams();
  const list = useMemo(() => upstreams.data ?? [], [upstreams.data]);
  const columns = useMemo(() => pickColumns(upstreamColumns({ wizard: true }), COLUMN_IDS), []);
  const selected = useMemo<ReadonlySet<string>>(() => (value === null ? new Set() : new Set([String(value)])), [value]);
  const onSelect = useCallback(
    (ids: Set<string>) => {
      const id = [...ids][0];
      if (id !== undefined) onChange(Number(id));
    },
    [onChange],
  );
  const rowSelection = useMemo(() => ({ mode: 'single' as const, selected, onChange: onSelect }), [selected, onSelect]);
  const status = useMemo(
    () => ({
      loading: upstreams.isLoading,
      error: upstreams.isError ? upstreams.error.message : null,
      empty: '还没有上游，请先新增上游',
    }),
    [upstreams.isLoading, upstreams.isError, upstreams.error],
  );

  return (
    <div className="grid gap-1">
      <p className="text-sm text-muted-foreground">选择要在哪个上游下创建渠道。</p>
      <DataTable<Upstream>
        layoutId="wizard.upstreams"
        className="max-h-72"
        data={list}
        columns={columns}
        getRowId={getUpstreamId}
        status={status}
        rowSelection={rowSelection}
      />
    </div>
  );
}
