import { useQuery } from '@tanstack/react-query';
import { NavLink } from 'react-router-dom';
import { api } from '@/api/endpoints';
import { Badge } from '@/components/ui/misc';
import { cn } from '@/lib/utils';
import { useChannelList } from '@/pages/channels/useChannels';
import { usePriceCounts } from '@/pages/pricing/usePrices';
import { NAV_GROUPS } from './nav';

export function Sidebar() {
  const upstreams = useQuery({ queryKey: ['upstreams'], queryFn: api.upstreams });
  const channels = useChannelList();
  const priceCounts = usePriceCounts();
  const attention = priceCounts.data ? priceCounts.data.no_price + priceCounts.data.deviation : undefined;
  const counts: Record<string, number | undefined> = {
    upstreams: upstreams.data?.length,
    channels: channels.data?.items.length,
    pricing: attention,
  };
  const draftCount = channels.data?.draft_count ?? 0;

  return (
    <aside className="flex w-48 shrink-0 flex-col border-r bg-card">
      <div className="flex h-11 items-center gap-2 border-b px-3">
        <span className="flex h-5 w-5 items-center justify-center rounded bg-primary text-xs font-bold text-primary-foreground">n</span>
        <span className="text-lg font-semibold">nam</span>
        <span className="text-xs text-muted-foreground">上游管理</span>
      </div>
      <nav className="flex-1 overflow-y-auto p-2">
        {NAV_GROUPS.map((group) => (
          <div key={group.title} className="mb-3">
            <div className="px-2 pb-1 text-xs text-muted-foreground">{group.title}</div>
            {group.items.map((item) => {
              const count = item.badge ? counts[item.badge] : undefined;
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === '/'}
                  className={({ isActive }) =>
                    cn(
                      'flex h-7 items-center gap-2 rounded px-2 text-base text-muted-foreground hover:bg-accent hover:text-foreground',
                      isActive && 'bg-accent font-medium text-foreground',
                    )
                  }
                >
                  <item.icon className="h-3.5 w-3.5 shrink-0" />
                  <span className="flex-1 truncate">{item.label}</span>
                  {item.reserved && <Badge>预留</Badge>}
                  {item.badge === 'channels' && draftCount > 0 && <Badge tone="warning" title="待应用草稿">{draftCount}</Badge>}
                  {count !== undefined && (
                    <Badge tone={item.badge === 'pricing' ? (count > 0 ? 'warning' : 'neutral') : 'accent'} title={item.badge === 'pricing' ? '缺价 + 偏差的模型数' : undefined}>
                      {count}
                    </Badge>
                  )}
                </NavLink>
              );
            })}
          </div>
        ))}
      </nav>
    </aside>
  );
}
