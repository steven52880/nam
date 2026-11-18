import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { useUpstreamTokens } from '@/api/queries';
import type { Upstream, UpstreamToken } from '@/api/types';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { AddTokenDialog, type TokenDialog } from '@/components/entities/AddTokenDialog';
import { tokenColumns } from '@/components/entities/tokenColumns';
import { customCol, DataTable, pickColumns } from '@/components/table';
import { Button } from '@/components/ui/button';
import { useActionMutation } from '@/hooks/useActionMutation';
import { UPSTREAM_CAPS } from '@/lib/labels';

const getTokenId = (t: UpstreamToken) => String(t.id);

export function TokensTab({ upstream }: { upstream: Upstream }) {
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<TokenDialog>(null);
  const [removing, setRemoving] = useState<UpstreamToken | null>(null);
  const tokens = useUpstreamTokens(upstream.id);
  const list = useMemo(() => tokens.data ?? [], [tokens.data]);
  const upstreamId = upstream.id;
  const canRemote = UPSTREAM_CAPS[upstream.type]?.remoteToken ?? false;

  const sync = useActionMutation({
    mutationFn: () => api.syncUpstream(upstream.id, ['tokens']),
    invalidate: (qc) => invalidate.upstream(qc),
    onSuccess: (results) => {
      const bad = results.find((r) => !r.ok);
      if (bad) toast.error(`同步令牌失败：${bad.error ?? ''}`);
      else toast.success('已同步令牌');
    },
  });
  const remove = useActionMutation({
    mutationFn: (id: number) => api.deleteToken(upstream.id, id),
    success: '已删除令牌记录',
    invalidate: (qc) => invalidate.tokens(qc, upstream.id),
  });

  const columns = useMemo(
    () => [
      ...pickColumns(tokenColumns(), ['name', 'key_hint', 'group', 'quota_usd', 'status', 'channel_count']),
      customCol<UpstreamToken>('actions', '操作', {
        width: 9,
        fixed: true,
        cell: (t) => (
          <span className="inline-flex items-center">
            {(t.channel_count ?? 0) === 0 && (
              <Button size="sm" variant="outline" onClick={() => navigate(`/channels?batch=${upstreamId}&token=${t.id}`)}>建渠道</Button>
            )}
            <Button size="sm" variant="ghost" className="text-destructive" onClick={() => setRemoving(t)}>删除</Button>
          </span>
        ),
      }),
    ],
    [navigate, upstreamId],
  );
  const status = useMemo(
    () => ({ loading: tokens.isLoading, error: tokens.isError ? tokens.error.message : null, empty: '还没有令牌' }),
    [tokens.isLoading, tokens.isError, tokens.error],
  );

  return (
    <div className="grid gap-2 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Button size="sm" variant="outline" disabled={sync.isPending} onClick={() => sync.mutate()}>{sync.isPending ? '同步中…' : '同步令牌'}</Button>
        <Button size="sm" variant="outline" onClick={() => setDialog('manual')}>手动录入</Button>
        {canRemote && <Button size="sm" variant="outline" onClick={() => setDialog('remote')}>在上游新建令牌</Button>}
        <span className="ml-auto text-sm text-muted-foreground">共 {list.length} 个</span>
      </div>
      <DataTable<UpstreamToken>
        layoutId="upstream.tokens"
        className="max-h-96"
        data={list}
        columns={columns}
        getRowId={getTokenId}
        status={status}
      />
      <AddTokenDialog upstream={upstream} mode={dialog} onClose={() => setDialog(null)} />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => { if (!open) setRemoving(null); }}
        title="删除令牌"
        description={`确定删除令牌「${removing?.name ?? ''}」？只删除 nam 里的记录。`}
        confirmLabel="确认删除"
        destructive
        onConfirm={() => {
          if (removing) remove.mutate(removing.id);
          setRemoving(null);
        }}
      />
    </div>
  );
}
