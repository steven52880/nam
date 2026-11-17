import * as DialogPrimitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import type { ComponentPropsWithoutRef, HTMLAttributes, ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './misc';

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

function Overlay({ className, ...props }: ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      className={cn(
        'fixed inset-0 z-50 bg-black/40 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
        className,
      )}
      {...props}
    />
  );
}

export type DialogSize = 'sm' | 'md' | 'lg' | 'xl' | '2xl' | '3xl' | '4xl';

/** 完整的类名字面量，Tailwind 才能扫描到。默认 lg = 原来写死的 max-w-lg。 */
const DIALOG_SIZES: Record<DialogSize, string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-xl',
  '2xl': 'max-w-2xl',
  '3xl': 'max-w-3xl',
  '4xl': 'max-w-4xl',
};

export function DialogContent({
  className,
  children,
  size = 'lg',
  ...props
}: ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & { size?: DialogSize }) {
  return (
    <DialogPrimitive.Portal>
      <Overlay />
      <DialogPrimitive.Content
        className={cn(
          'fixed left-1/2 top-1/2 z-50 flex max-h-[90vh] w-full -translate-x-1/2 -translate-y-1/2 flex-col gap-3 rounded-lg border bg-popover p-4 shadow-lg data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
          DIALOG_SIZES[size],
          className,
        )}
        {...props}
      >
        {children}
        <DialogPrimitive.Close className="absolute right-3 top-3 rounded p-0.5 text-muted-foreground hover:bg-accent">
          <X className="h-4 w-4" />
          <span className="sr-only">关闭</span>
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

export function DialogHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex flex-col gap-1', className)} {...props} />;
}

/** 弹窗里唯一的滚动区；标题和按钮区不滚动。 */
export function DialogBody({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('min-h-0 flex-1 overflow-y-auto', className)} {...props} />;
}

export function DialogFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex justify-end gap-2 pt-1', className)} {...props} />;
}

export function DialogTitle({ className, ...props }: ComponentPropsWithoutRef<typeof DialogPrimitive.Title>) {
  return <DialogPrimitive.Title className={cn('text-lg font-semibold', className)} {...props} />;
}

export function DialogDescription({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof DialogPrimitive.Description>) {
  return <DialogPrimitive.Description className={cn('text-sm text-muted-foreground', className)} {...props} />;
}

/** 右侧抽屉，基于 Dialog。页面请优先使用 DrawerShell。 */
export function SheetContent({
  className,
  children,
  ...props
}: ComponentPropsWithoutRef<typeof DialogPrimitive.Content>) {
  return (
    <DialogPrimitive.Portal>
      <Overlay />
      <DialogPrimitive.Content
        className={cn(
          'fixed inset-y-0 right-0 z-50 flex w-full max-w-xl flex-col border-l bg-popover shadow-lg data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right',
          className,
        )}
        {...props}
      >
        {children}
        <DialogPrimitive.Close className="absolute right-3 top-3 rounded p-0.5 text-muted-foreground hover:bg-accent">
          <X className="h-4 w-4" />
          <span className="sr-only">关闭</span>
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

export interface DrawerTab {
  value: string;
  label: string;
  content: ReactNode;
  disabled?: boolean;
}

export interface DrawerShellProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  description?: ReactNode;
  /** 默认 xl（= SheetContent 的 max-w-xl）。 */
  size?: 'xl' | '2xl';
  actions?: ReactNode;
  tabs?: DrawerTab[];
  defaultTab?: string;
  /** 没有 tabs 时的正文。 */
  children?: ReactNode;
}

const DRAWER_SIZES = { xl: 'max-w-xl', '2xl': 'max-w-2xl' } as const;

/** 右侧抽屉：标题区 + 可选标签页 + 唯一的滚动正文区。 */
export function DrawerShell({
  open,
  onOpenChange,
  title,
  description,
  size = 'xl',
  actions,
  tabs,
  defaultTab,
  children,
}: DrawerShellProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <SheetContent className={DRAWER_SIZES[size]}>
        <div className="flex items-start gap-2 border-b px-4 py-3">
          <div className="min-w-0 flex-1">
            <DialogTitle>{title}</DialogTitle>
            {description && <DialogDescription className="truncate">{description}</DialogDescription>}
          </div>
          {actions && <div className="mr-6 flex shrink-0 items-center gap-2">{actions}</div>}
        </div>
        {tabs && tabs.length > 0 ? (
          <Tabs defaultValue={defaultTab ?? tabs[0].value} className="flex min-h-0 flex-1 flex-col">
            <TabsList className="px-3">
              {tabs.map((t) => (
                <TabsTrigger key={t.value} value={t.value} disabled={t.disabled}>{t.label}</TabsTrigger>
              ))}
            </TabsList>
            <div className="min-h-0 flex-1 overflow-y-auto">
              {tabs.map((t) => (
                <TabsContent key={t.value} value={t.value}>{t.content}</TabsContent>
              ))}
            </div>
          </Tabs>
        ) : (
          <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
        )}
      </SheetContent>
    </Dialog>
  );
}
