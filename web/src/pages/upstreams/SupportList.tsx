import { Check, Minus, X } from 'lucide-react';
import type { Upstream } from '@/api/types';
import { syncPartViews, type SyncPartView } from '@/lib/syncParts';
import { cn } from '@/lib/utils';

const ICON_TONE: Record<SyncPartView['status'], string> = {
  ok: 'text-success',
  bad: 'text-destructive',
  none: 'text-muted-foreground',
};

function Item({ v }: { v: SyncPartView }) {
  const Icon = v.status === 'ok' ? Check : v.status === 'bad' ? X : Minus;
  return (
    <li className="flex items-start gap-2 py-0.5">
      <Icon className={cn('mt-0.5 h-3.5 w-3.5 shrink-0', ICON_TONE[v.status])} aria-label={v.status === 'ok' ? '成功' : v.status === 'bad' ? '失败' : '未同步'} />
      <span className="w-20 shrink-0 font-medium">{v.label}</span>
      <span className="min-w-0 flex-1">
        {v.reason && (
          <span className={v.status === 'bad' ? 'text-destructive' : 'text-muted-foreground'}>{v.reason}</span>
        )}
        {v.countText && <span className={cn('tabular-nums text-muted-foreground', v.reason && 'ml-2')}>{v.countText}</span>}
        {v.status === 'ok' && !v.countText && <span className="text-muted-foreground">正常</span>}
      </span>
      {v.agoText && <span className="shrink-0 tabular-nums text-xs text-muted-foreground">{v.agoText}</span>}
    </li>
  );
}

/** 上游抽屉概览里的「支持情况」：绿勾 = 成功；红叉 = 失败 / 不支持 / 没有数据；灰色 = 还没同步。 */
export function SupportList({ upstream }: { upstream: Upstream }) {
  const views = syncPartViews(upstream.sync_parts);
  return (
    <div className="rounded border bg-muted/30 px-3 py-2">
      <div className="mb-0.5 text-sm text-muted-foreground">支持情况</div>
      <ul>{views.map((v) => <Item key={v.key} v={v} />)}</ul>
    </div>
  );
}
