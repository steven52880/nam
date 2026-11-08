import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { Button } from '@/components/ui/button';
import { formatNumber, formatTime } from '@/lib/format';
import { DeriveSection } from './DeriveSection';

export function GroupsTab({ upstreamId }: { upstreamId: number }) {
  const qc = useQueryClient();
  const groups = useQuery({ queryKey: ['upstream-groups', upstreamId], queryFn: () => api.upstreamGroups(upstreamId) });
  const sync = useMutation({
    mutationFn: () => api.syncUpstream(upstreamId, ['groups']),
    onSuccess: (results) => {
      const bad = results.find((r) => !r.ok);
      if (bad) toast.error(`同步倍率失败：${bad.error ?? ''}`);
      else toast.success('已同步分组倍率');
      void qc.invalidateQueries({ queryKey: ['upstream-groups', upstreamId] });
      void qc.invalidateQueries({ queryKey: ['upstreams'] });
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });
  const list = groups.data ?? [];

  return (
    <div className="grid gap-2 p-4">
      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">共 {list.length} 个分组</span>
        <Button size="sm" disabled={sync.isPending} onClick={() => sync.mutate()}>{sync.isPending ? '同步中…' : '立即同步倍率'}</Button>
      </div>
      <div className="overflow-auto rounded border">
        <table className="w-full text-base">
          <thead className="bg-muted text-left text-sm text-muted-foreground">
            <tr><th className="h-7 px-2 font-medium">分组名</th><th className="px-2 text-right font-medium">分组倍率</th><th className="px-2 font-medium">说明</th><th className="px-2 font-medium">更新时间</th></tr>
          </thead>
          <tbody>
            {list.map((g) => (
              <tr key={g.name} className="h-7 border-t">
                <td className="px-2 font-medium">{g.name}</td>
                <td className="px-2 text-right tabular-nums">{formatNumber(g.ratio, 3)}</td>
                <td className="px-2 text-muted-foreground">{g.desc || '—'}</td>
                <td className="whitespace-nowrap px-2 tabular-nums">{formatTime(g.updated_at)}</td>
              </tr>
            ))}
            {groups.isLoading && <tr><td colSpan={4} className="h-12 text-center text-muted-foreground">加载中…</td></tr>}
            {groups.isSuccess && list.length === 0 && <tr><td colSpan={4} className="h-12 text-center text-muted-foreground">还没有分组倍率，点击「立即同步倍率」</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export function ModelsTab({ upstreamId }: { upstreamId: number }) {
  const models = useQuery({ queryKey: ['upstream-models', upstreamId], queryFn: () => api.upstreamModels(upstreamId) });
  const list = models.data ?? [];
  const num = (v: number | null) => (v === null ? '—' : formatNumber(v, 4));

  return (
    <div className="grid gap-2 p-4">
      <DeriveSection upstreamId={upstreamId} />
      <span className="text-sm text-muted-foreground">共 {list.length} 条上游模型价格（美元 / 百万，按上游额度计，已换算到各分组）</span>
      <div className="max-h-[60vh] overflow-auto rounded border">
        <table className="w-max min-w-full text-base">
          <thead className="sticky top-0 bg-muted text-left text-sm text-muted-foreground">
            <tr>
              {['模型', '分组', '倍率', '输入', '输出', '缓存读', '缓存写', '按次'].map((h) => <th key={h} className="h-7 whitespace-nowrap px-2 font-medium">{h}</th>)}
            </tr>
          </thead>
          <tbody>
            {list.map((m) => (
              <tr key={m.id} className="h-7 border-t">
                <td className="px-2 font-medium">{m.model}</td>
                <td className="px-2">{m.group || '—'}</td>
                <td className="px-2 text-right tabular-nums">{num(m.model_ratio)}</td>
                <td className="px-2 text-right tabular-nums">{num(m.input)}</td>
                <td className="px-2 text-right tabular-nums">{num(m.output)}</td>
                <td className="px-2 text-right tabular-nums">{num(m.cache_read)}</td>
                <td className="px-2 text-right tabular-nums">{num(m.cache_write)}</td>
                <td className="px-2 text-right tabular-nums">{num(m.per_request)}</td>
              </tr>
            ))}
            {models.isLoading && <tr><td colSpan={8} className="h-12 text-center text-muted-foreground">加载中…</td></tr>}
            {models.isSuccess && list.length === 0 && <tr><td colSpan={8} className="h-12 text-center text-muted-foreground">暂无上游模型价格（价格同步在后续里程碑提供）</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  );
}
