import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type BannerTone = 'error' | 'warning' | 'info';

const TONES: Record<BannerTone, string> = {
  error: 'bg-destructive/10 text-destructive',
  warning: 'bg-warning/15 text-warning',
  info: 'bg-muted text-muted-foreground',
};

export function Banner({
  tone,
  children,
  action,
  className,
}: {
  tone: BannerTone;
  children: ReactNode;
  /** 右侧的操作按钮。 */
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'rounded px-2 py-1 text-sm',
        action && 'flex items-center justify-between gap-2',
        TONES[tone],
        className,
      )}
    >
      {action ? <div className="min-w-0 flex-1">{children}</div> : children}
      {action}
    </div>
  );
}
