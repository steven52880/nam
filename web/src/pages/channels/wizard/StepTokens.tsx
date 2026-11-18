import { useCallback, useMemo, useState } from 'react';
import { useUpstreamGroups, useUpstreams, useUpstreamTokens, useUsdCnyRate } from '@/api/queries';
import type { UpstreamToken } from '@/api/types';
import { AddTokenDialog, type TokenDialog } from '@/components/entities/AddTokenDialog';
import { tokenColumns } from '@/components/entities/tokenColumns';
import { DataTable, pickColumns } from '@/components/table';
import { Button } from '@/components/ui/button';
import { UPSTREAM_CAPS } from '@/lib/labels';

const getTokenId = (t: UpstreamToken) => String(t.id);
const COLUMN_IDS = ['name', 'key_hint', 'group', 'group_ratio', 'price_ratio', 'quota_usd', 'status', 'channel_count'];

interface StepTokensProps {
  upstreamId: number;
  value: number[];
  onChange: (ids: number[]) => void;
  /** 单个渠道模式：只能选一个令牌。 */
  single?: boolean;
}

export function StepTokens({ upstreamId, value, onChange, single = false }: StepTokensProps) {
  const [dialog, setDialog] = useState<TokenDialog>(null);
  const upstreams = useUpstreams();
  const tokens = useUpstreamTokens(upstreamId);
  const groups = useUpstreamGroups(upstreamId);
  const upstream = upstreams.data?.find((u) => u.id === upstreamId) ?? null;
  const list = useMemo(() => tokens.data ?? [], [tokens.data]);
  const groupList = groups.data;
  const rate = upstream?.currency_rate;
  const usdRate = useUsdCnyRate();
  const columns = useMemo(
    () => pickColumns(tokenColumns({ groups: groupList, rate, usdRate }), COLUMN_IDS),
    [groupList, rate, usdRate],
  );
  const selected = useMemo<ReadonlySet<string>>(() => new Set(value.map(String)), [value]);
  const toggle = useCallback(
    (id: number) => {
      if (single) onChange(value.includes(id) ? [] : [id]);
      else onChange(value.includes(id) ? value.filter((v) => v !== id) : [...value, id]);
    },
    [single, value, onChange],
  );
  // 整行点击切换选中：单选模式下 DataTable 会先选中再回调这里，同一次事件里以最后一次 onChange 为准，保持原来"再点已选行取消"的行为。
  const onRowClick = useCallback((t: UpstreamToken) => toggle(t.id), [toggle]);
  const onSelectionChange = useCallback((ids: Set<string>) => onChange([...ids].map(Number)), [onChange]);
  const rowSelection = useMemo(
    () => ({ mode: single ? ('single' as const) : ('multi' as const), selected, onChange: onSelectionChange }),
    [single, selected, onSelectionChange],
  );
  const status = useMemo(
    () => ({
      loading: tokens.isLoading,
      error: tokens.isError ? tokens.error.message : null,
      empty: '该上游还没有令牌，请同步、在上游新建或手动录入',
    }),
    [tokens.isLoading, tokens.isError, tokens.error],
  );
  const unbuilt = list.filter((t) => (t.channel_count ?? 0) === 0).map((t) => t.id);

  return (
    <div className="grid gap-2">
      <div className="flex flex-wrap items-center gap-2">
        {!single && (
          <>
            <Button size="sm" variant="outline" onClick={() => onChange(unbuilt)}>全选未建渠道的（{unbuilt.length}）</Button>
            <Button size="sm" variant="outline" onClick={() => onChange([])}>清空</Button>
          </>
        )}
        <span className="text-sm text-muted-foreground">
          {single ? '单个渠道模式：只能选一个令牌，' : ''}已选 {value.length} 个令牌
        </span>
        {upstream && (
          <span className="ml-auto flex gap-2">
            {UPSTREAM_CAPS[upstream.type]?.remoteToken && (
              <Button size="sm" variant="outline" onClick={() => setDialog('remote')}>在上游新建令牌</Button>
            )}
            <Button size="sm" variant="outline" onClick={() => setDialog('manual')}>手动录入 key</Button>
          </span>
        )}
      </div>
      <DataTable<UpstreamToken>
        layoutId="wizard.tokens"
        className="max-h-72"
        data={list}
        columns={columns}
        getRowId={getTokenId}
        status={status}
        rowSelection={rowSelection}
        onRowClick={onRowClick}
      />
      {upstream && (
        <AddTokenDialog
          upstream={upstream}
          mode={dialog}
          onClose={() => setDialog(null)}
          onCreated={(token) => onChange(single ? [token.id] : [...value.filter((v) => v !== token.id), token.id])}
        />
      )}
    </div>
  );
}
