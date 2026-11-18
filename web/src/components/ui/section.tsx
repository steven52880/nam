import type { HTMLAttributes, ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type SectionProps = {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  /** card：设置页卡片；plain：抽屉里的小标题段落。 */
  variant?: 'card' | 'plain';
  /** card 时正文贴边（不加内边距和间距），用于里面直接放表格或列表。 */
  flush?: boolean;
} & Omit<HTMLAttributes<HTMLElement>, 'title'>;

export function Section({ title, description, actions, variant = 'card', flush, className, children, ...props }: SectionProps) {
  if (variant === 'plain') {
    return (
      <section className={cn('grid grid-cols-[minmax(0,1fr)] gap-1', className)} {...props}>
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-sm font-medium text-muted-foreground">{title}</h3>
          {actions}
        </div>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
        {children}
      </section>
    );
  }
  return (
    <section className={cn('scroll-mt-4 rounded border bg-card', className)} {...props}>
      <div className="flex h-9 items-center gap-2 border-b px-3 font-medium">
        <span>{title}</span>
        {description && <span className="text-sm font-normal text-muted-foreground">{description}</span>}
        {actions && <div className="ml-auto flex items-center gap-2">{actions}</div>}
      </div>
      {flush ? children : <div className="grid grid-cols-[minmax(0,1fr)] gap-3 p-3">{children}</div>}
    </section>
  );
}

/** 左侧标签、右侧内容的一行（设置页的 Line）。labelClassName 可覆盖标签宽度（默认 w-40）。 */
export function DescRow({
  label,
  hint,
  labelClassName,
  children,
}: {
  label: ReactNode;
  hint?: ReactNode;
  labelClassName?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex items-center gap-3">
      <span className={cn('w-40 shrink-0 text-muted-foreground', labelClassName)}>{label}</span>
      <div className="min-w-0 flex-1">
        {children}
        {hint && <span className="ml-2 text-sm text-muted-foreground">{hint}</span>}
      </div>
    </div>
  );
}
