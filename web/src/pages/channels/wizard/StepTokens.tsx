import { useState } from 'react';
import { groupRatioOf, useUpstreamGroups, useUpstreams, useUpstreamTokens } from '@/api/queries';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { AddTokenDialog, type TokenDialog } from '@/components/entities/AddTokenDialog';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { formatNumber } from '@/lib/format';
import { TOKEN_STATUS, UPSTREAM_CAPS } from '@/lib/labels';

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
  const list = tokens.data ?? [];
  const ratioOf = (group: string) => groupRatioOf(groups.data, group);
  const toggle = (id: number) => {
    if (single) onChange(value.includes(id) ? [] : [id]);
    else onChange(value.includes(id) ? value.filter((v) => v !== id) : [...value, id]);
  };
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
      <div className="max-h-72 overflow-y-auto rounded border">
        <table className="w-full text-base">
          <thead className="sticky top-0 bg-muted text-left text-sm text-muted-foreground">
            <tr>
              <th className="h-7 w-8 px-2" />
              <th className="px-2 font-medium">令牌</th>
              <th className="px-2 font-medium">上游分组</th>
              <th className="px-2 text-right font-medium">分组倍率</th>
              <th className="px-2 text-right font-medium">额度（美元）</th>
              <th className="px-2 font-medium">状态</th>
              <th className="px-2 font-medium">渠道</th>
            </tr>
          </thead>
          <tbody>
            {list.map((t) => (
              <tr key={t.id} className="h-7 cursor-pointer border-t hover:bg-accent/40" onClick={() => toggle(t.id)}>
                <td className="px-2"><input type={single ? 'radio' : 'checkbox'} checked={value.includes(t.id)} readOnly /></td>
                <td className="px-2"><span className="font-medium">{t.name}</span> <span className="text-xs text-muted-foreground">{t.key_hint}</span></td>
                <td className="px-2">{t.group || '—'}</td>
                <td className="px-2 text-right tabular-nums">{formatNumber(ratioOf(t.group), 3)}</td>
                <td className="px-2 text-right tabular-nums">{t.unlimited ? '不限' : formatNumber(t.quota_usd)}</td>
                <td className="px-2"><StatusBadge dict={TOKEN_STATUS} value={t.status === 1 ? 1 : 0} /></td>
                <td className="px-2">{(t.channel_count ?? 0) > 0 ? <Badge tone="accent">已建 {t.channel_count}</Badge> : <span className="text-muted-foreground">未建</span>}</td>
              </tr>
            ))}
            {tokens.isLoading && <tr><td colSpan={7} className="h-12 text-center text-muted-foreground">加载中…</td></tr>}
            {tokens.isSuccess && list.length === 0 && <tr><td colSpan={7} className="h-12 text-center text-muted-foreground">该上游还没有令牌，请同步、在上游新建或手动录入</td></tr>}
          </tbody>
        </table>
      </div>
      {upstream && (
        <AddTokenDialog
          upstream={upstream}
          mode={dialog}
          groups={groups.data}
          onClose={() => setDialog(null)}
          onCreated={(token) => onChange(single ? [token.id] : [...value.filter((v) => v !== token.id), token.id])}
        />
      )}
    </div>
  );
}
