import { api } from '@/api/endpoints';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { useActionMutation } from '@/hooks/useActionMutation';
import { formatNumber, formatPercent } from '@/lib/format';

/** 推算折扣：按各渠道的上游价推算上游分组折扣，并标出偏离超过 5% 的模型。 */
export function DeriveSection({ upstreamId }: { upstreamId: number }) {
  const derive = useActionMutation({ mutationFn: () => api.deriveUpstream(upstreamId) });
  const channels = derive.data ?? [];

  return (
    <div className="grid gap-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm text-muted-foreground">
          偏离渠道折扣超过 5% 的模型会被标出；单个模型的折扣覆盖请在渠道页的模型明细中设置。
        </span>
        <Button size="sm" variant="outline" disabled={derive.isPending} onClick={() => derive.mutate()}>
          {derive.isPending ? '推算中…' : '推算折扣'}
        </Button>
      </div>
      {derive.isSuccess && channels.length === 0 && (
        <div className="rounded border border-dashed py-4 text-center text-muted-foreground">该上游下没有可推算的渠道</div>
      )}
      {channels.map((channel) => {
        const { derivation } = channel;
        const flagged = derivation.models.filter((m) => m.deviates);
        return (
          <div key={channel.channel_id} className="rounded border">
            <div className="flex items-center gap-3 border-b bg-muted/50 px-2 py-1 text-sm">
              <span className="font-medium">{channel.name}</span>
              <span className="text-muted-foreground">推算折扣中位数：{formatNumber(derivation.median, 3)}</span>
              <Badge tone={flagged.length > 0 ? 'warning' : 'success'}>
                {flagged.length > 0 ? <ModelCountCell count={flagged.length} variant="deviated" /> : '无偏离'}
              </Badge>
            </div>
            {flagged.length > 0 && (
              <table className="w-full text-base">
                <tbody>
                  {flagged.map((m) => (
                    <tr key={m.model} className="h-7 border-t first:border-t-0">
                      <td className="px-2">{m.model}</td>
                      <td className="px-2 text-right tabular-nums">{formatNumber(m.discount, 3)}</td>
                      <td className="px-2 text-right tabular-nums text-destructive">{formatPercent(m.deviation, { signed: true })}</td>
                      <td className="px-2 text-sm text-muted-foreground">{m.reason}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        );
      })}
    </div>
  );
}
