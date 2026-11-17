import type { CSSProperties, ReactNode } from 'react';

export const COLORS = {
  primary: 'hsl(var(--primary))',
  success: 'hsl(var(--success))',
  warning: 'hsl(var(--warning))',
  danger: 'hsl(var(--destructive))',
  muted: 'hsl(var(--muted-foreground))',
};

export const AXIS_TICK = { fontSize: '0.6875rem', fill: 'hsl(var(--muted-foreground))' };
export const GRID_STROKE = 'hsl(var(--border))';
export const CHART_MARGIN = { top: 6, right: 8, bottom: 0, left: -8 };

const TOOLTIP_STYLE: CSSProperties = {
  background: 'hsl(var(--popover))',
  border: '1px solid hsl(var(--border))',
  borderRadius: '0.25rem',
  fontSize: '0.75rem',
  padding: '0.25rem 0.5rem',
};

export const TOOLTIP_PROPS = {
  contentStyle: TOOLTIP_STYLE,
  labelStyle: { color: 'hsl(var(--foreground))' },
  itemStyle: { color: 'hsl(var(--foreground))', padding: 0 },
};

export const BAR_CURSOR = { fill: 'hsl(var(--accent))', opacity: 0.4 };

export function ChartCard({ title, empty, children }: { title: string; empty: boolean; children: ReactNode }) {
  return (
    <div className="rounded border bg-card p-2">
      <div className="mb-1 text-sm font-medium">{title}</div>
      <div className="h-56">
        {empty ? (
          <div className="flex h-full items-center justify-center text-muted-foreground">暂无数据</div>
        ) : (
          children
        )}
      </div>
    </div>
  );
}
