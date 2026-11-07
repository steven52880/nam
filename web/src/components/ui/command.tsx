import { Command as CommandPrimitive } from 'cmdk';
import { Search } from 'lucide-react';
import type { ComponentPropsWithoutRef } from 'react';
import { Dialog, DialogContent, DialogTitle } from './dialog';
import { cn } from '@/lib/utils';

export function CommandDialog({
  children,
  ...props
}: ComponentPropsWithoutRef<typeof Dialog>) {
  return (
    <Dialog {...props}>
      <DialogContent className="top-[20%] max-w-xl translate-y-0 gap-0 overflow-hidden p-0">
        <DialogTitle className="sr-only">全局搜索</DialogTitle>
        <CommandPrimitive className="flex flex-col">{children}</CommandPrimitive>
      </DialogContent>
    </Dialog>
  );
}

export function CommandInput({ className, ...props }: ComponentPropsWithoutRef<typeof CommandPrimitive.Input>) {
  return (
    <div className="flex items-center gap-2 border-b px-3">
      <Search className="h-4 w-4 shrink-0 opacity-60" />
      <CommandPrimitive.Input
        className={cn('h-9 w-full bg-transparent text-base outline-none placeholder:text-muted-foreground', className)}
        {...props}
      />
    </div>
  );
}

export function CommandList({ className, ...props }: ComponentPropsWithoutRef<typeof CommandPrimitive.List>) {
  return <CommandPrimitive.List className={cn('max-h-80 overflow-y-auto p-1', className)} {...props} />;
}

export function CommandEmpty(props: ComponentPropsWithoutRef<typeof CommandPrimitive.Empty>) {
  return <CommandPrimitive.Empty className="py-6 text-center text-base text-muted-foreground" {...props} />;
}

export function CommandGroup({ className, ...props }: ComponentPropsWithoutRef<typeof CommandPrimitive.Group>) {
  return (
    <CommandPrimitive.Group
      className={cn(
        'overflow-hidden text-foreground [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:text-muted-foreground',
        className,
      )}
      {...props}
    />
  );
}

export function CommandItem({ className, ...props }: ComponentPropsWithoutRef<typeof CommandPrimitive.Item>) {
  return (
    <CommandPrimitive.Item
      className={cn(
        'flex h-7 cursor-default select-none items-center gap-2 rounded px-2 text-base outline-none data-[selected=true]:bg-accent',
        className,
      )}
      {...props}
    />
  );
}
