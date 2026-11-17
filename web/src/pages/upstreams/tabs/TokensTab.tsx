import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { useUpstreamTokens } from '@/api/queries';
import type { Upstream, UpstreamToken } from '@/api/types';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { useActionMutation } from '@/hooks/useActionMutation';
import { formatNumber } from '@/lib/format';
import { TOKEN_STATUS, UPSTREAM_CAPS } from '@/lib/labels';
import { AddTokenDialog, type TokenDialog } from '@/components/entities/AddTokenDialog';

export function TokensTab({ upstream }: { upstream: Upstream }) {
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<TokenDialog>(null);
  const [removing, setRemoving] = useState<UpstreamToken | null>(null);
  const tokens = useUpstreamTokens(upstream.id);
  const list = tokens.data ?? [];
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

  return (
    <div className="grid gap-2 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Button size="sm" variant="outline" disabled={sync.isPending} onClick={() => sync.mutate()}>{sync.isPending ? '同步中…' : '同步令牌'}</Button>
        <Button size="sm" variant="outline" onClick={() => setDialog('manual')}>手动录入</Button>
        {canRemote && <Button size="sm" variant="outline" onClick={() => setDialog('remote')}>在上游新建令牌</Button>}
        <span className="ml-auto text-sm text-muted-foreground">共 {list.length} 个</span>
      </div>
      <div className="overflow-auto rounded border">
        <table className="w-full text-base">
          <thead className="bg-muted text-left text-sm text-muted-foreground">
            <tr>
              {['令牌名', 'key', '分组', '额度（美元）', '状态', '渠道', ''].map((h) => <th key={h || 'op'} className="h-7 whitespace-nowrap px-2 font-medium">{h}</th>)}
            </tr>
          </thead>
          <tbody>
            {list.map((t) => (
              <tr key={t.id} className="h-8 border-t">
                <td className="px-2 font-medium">{t.name}{t.manual && <Badge className="ml-1">手动</Badge>}</td>
                <td className="px-2 text-muted-foreground">{t.key_hint || '—'}</td>
                <td className="px-2">{t.group || '—'}</td>
                <td className="px-2 text-right tabular-nums">{t.unlimited ? '不限' : formatNumber(t.quota_usd)}</td>
                <td className="px-2"><StatusBadge dict={TOKEN_STATUS} value={t.status === 1 ? 1 : 0} /></td>
                <td className="px-2">{(t.channel_count ?? 0) > 0 ? <Badge tone="accent">已建 {t.channel_count}</Badge> : <span className="text-muted-foreground">未建</span>}</td>
                <td className="whitespace-nowrap px-2 text-right">
                  {(t.channel_count ?? 0) === 0 && (
                    <Button size="sm" variant="outline" onClick={() => navigate(`/channels?batch=${upstream.id}&token=${t.id}`)}>建渠道</Button>
                  )}
                  <Button size="sm" variant="ghost" className="text-destructive" onClick={() => setRemoving(t)}>删除</Button>
                </td>
              </tr>
            ))}
            {tokens.isLoading && <tr><td colSpan={7} className="h-12 text-center text-muted-foreground">加载中…</td></tr>}
            {tokens.isSuccess && list.length === 0 && <tr><td colSpan={7} className="h-12 text-center text-muted-foreground">还没有令牌</td></tr>}
          </tbody>
        </table>
      </div>
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
