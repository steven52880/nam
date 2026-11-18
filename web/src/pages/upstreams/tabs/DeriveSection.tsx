import { useMemo } from 'react';
import { api } from '@/api/endpoints';
import type { DeriveChannel, DeriveModel } from '@/api/types';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { DataTable, numberCol, textCol, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { useActionMutation } from '@/hooks/useActionMutation';
import { formatNumber, formatPercent } from '@/lib/format';

/** 推算倍率：按各渠道的上游价推算上游分组倍率，并标出偏离超过 5% 的模型。 */
export function DeriveSection({ upstreamId }: { upstreamId: number }) {
  const derive = useActionMutation({ mutationFn: () => api.deriveUpstream(upstreamId) });
  const channels = derive.data ?? [];

  return (
    <div className="grid gap-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm text-muted-foreground">
          偏离渠道倍率超过 5% 的模型会被标出；单个模型的倍率覆盖请在渠道页的模型明细中设置。
        </span>
        <Button size="sm" variant="outline" disabled={derive.isPending} onClick={() => derive.mutate()}>
          {derive.isPending ? '推算中…' : '推算倍率'}
        </Button>
      </div>
      {derive.isSuccess && channels.length === 0 && (
        <div className="rounded border border-dashed py-4 text-center text-muted-foreground">该上游下没有可推算的渠道</div>
      )}
      {channels.map((channel) => (
        <ChannelDerivation key={channel.channel_id} channel={channel} />
      ))}
    </div>
  );
}

const getModelKey = (m: DeriveModel) => m.model;

/** 偏离明细的列；列宽单位 rem，列 id 上线后不改。 */
function deriveColumns(): TableColumn<DeriveModel>[] {
  return [
    textCol<DeriveModel>('model', '模型', (m) => m.model, { width: 14 }),
    numberCol<DeriveModel>('discount', '推算倍率', (m) => m.discount, {
      width: 6.5,
      cell: (m) => formatNumber(m.discount, 3),
    }),
    numberCol<DeriveModel>('deviation', '偏离', (m) => m.deviation, {
      width: 6,
      className: 'text-destructive',
      cell: (m) => formatPercent(m.deviation, { signed: true }),
    }),
    textCol<DeriveModel>('reason', '原因', (m) => m.reason, { width: 16, className: 'text-sm text-muted-foreground' }),
  ];
}

function ChannelDerivation({ channel }: { channel: DeriveChannel }) {
  const { derivation } = channel;
  const flagged = useMemo(() => derivation.models.filter((m) => m.deviates), [derivation.models]);
  const columns = useMemo(() => deriveColumns(), []);
  return (
    <div className="rounded border">
      <div className="flex items-center gap-3 border-b bg-muted/50 px-2 py-1 text-sm">
        <span className="font-medium">{channel.name}</span>
        <span className="text-muted-foreground">推算倍率中位数：{formatNumber(derivation.median, 3)}</span>
        <Badge tone={flagged.length > 0 ? 'warning' : 'success'}>
          {flagged.length > 0 ? <ModelCountCell count={flagged.length} variant="deviated" /> : '无偏离'}
        </Badge>
      </div>
      {flagged.length > 0 && (
        <DataTable<DeriveModel>
          layoutId="upstream.derive"
          className="max-h-60 rounded-none border-0"
          data={flagged}
          columns={columns}
          getRowId={getModelKey}
        />
      )}
    </div>
  );
}
