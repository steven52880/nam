import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/endpoints';
import { cn } from '@/lib/utils';
import { formatNumber } from '@/lib/format';

export function StepUpstream({ value, onChange }: { value: number | null; onChange: (id: number) => void }) {
  const upstreams = useQuery({ queryKey: ['upstreams'], queryFn: api.upstreams });
  const list = upstreams.data ?? [];
  return (
    <div className="grid gap-1">
      <p className="text-sm text-muted-foreground">选择要在哪个上游下创建渠道。</p>
      <div className="max-h-72 overflow-y-auto rounded border">
        <table className="w-full text-base">
          <thead className="sticky top-0 bg-muted text-left text-sm text-muted-foreground">
            <tr>
              <th className="h-7 px-2 font-medium">名称</th>
              <th className="px-2 font-medium">类型</th>
              <th className="px-2 text-right font-medium">上游货币汇率</th>
              <th className="px-2 text-right font-medium">令牌数</th>
              <th className="px-2 text-right font-medium">已建渠道数</th>
            </tr>
          </thead>
          <tbody>
            {list.map((u) => (
              <tr
                key={u.id}
                onClick={() => onChange(u.id)}
                className={cn('h-7 cursor-pointer border-t hover:bg-accent/40', value === u.id && 'bg-primary/15')}
              >
                <td className="px-2 font-medium">{u.name}</td>
                <td className="px-2">{u.type}</td>
                <td className="px-2 text-right tabular-nums">{formatNumber(u.currency_rate)}</td>
                <td className="px-2 text-right tabular-nums">{u.token_count}</td>
                <td className="px-2 text-right tabular-nums">{u.channel_count}</td>
              </tr>
            ))}
            {upstreams.isLoading && <tr><td colSpan={5} className="h-12 text-center text-muted-foreground">加载中…</td></tr>}
            {upstreams.isSuccess && list.length === 0 && <tr><td colSpan={5} className="h-12 text-center text-muted-foreground">还没有上游，请先新增上游</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  );
}
