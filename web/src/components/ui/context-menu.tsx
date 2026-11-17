import * as ContextPrimitive from '@radix-ui/react-context-menu';
import type { ComponentPropsWithoutRef } from 'react';
import { cn } from '@/lib/utils';

export const ContextMenu = ContextPrimitive.Root;
export const ContextMenuTrigger = ContextPrimitive.Trigger;
export const ContextMenuSub = ContextPrimitive.Sub;

export function ContextMenuContent({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof ContextPrimitive.Content>) {
  return (
    <ContextPrimitive.Portal>
      <ContextPrimitive.Content
        className={cn('z-[60] min-w-[10rem] rounded-md border bg-popover p-1 shadow-md', className)}
        {...props}
      />
    </ContextPrimitive.Portal>
  );
}

export function ContextMenuItem({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof ContextPrimitive.Item>) {
  return (
    <ContextPrimitive.Item
      className={cn(
        'flex h-7 cursor-default select-none items-center gap-2 rounded px-2 text-base outline-none data-[highlighted]:bg-accent data-[disabled]:opacity-50',
        className,
      )}
      {...props}
    />
  );
}

export function ContextMenuCheckboxItem({
  className,
  children,
  ...props
}: ComponentPropsWithoutRef<typeof ContextPrimitive.CheckboxItem>) {
  return (
    <ContextPrimitive.CheckboxItem
      className={cn(
        'relative flex h-7 cursor-default select-none items-center rounded pl-6 pr-2 text-base outline-none data-[highlighted]:bg-accent data-[disabled]:opacity-50',
        className,
      )}
      {...props}
    >
      <span className="absolute left-2 text-xs">
        <ContextPrimitive.ItemIndicator>✓</ContextPrimitive.ItemIndicator>
      </span>
      {children}
    </ContextPrimitive.CheckboxItem>
  );
}

export function ContextMenuLabel({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof ContextPrimitive.Label>) {
  return <ContextPrimitive.Label className={cn('px-2 py-1 text-sm text-muted-foreground', className)} {...props} />;
}

export function ContextMenuSeparator({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof ContextPrimitive.Separator>) {
  return <ContextPrimitive.Separator className={cn('-mx-1 my-1 h-px bg-border', className)} {...props} />;
}

export function ContextMenuSubTrigger({
  className,
  children,
  ...props
}: ComponentPropsWithoutRef<typeof ContextPrimitive.SubTrigger>) {
  return (
    <ContextPrimitive.SubTrigger
      className={cn(
        'flex h-7 cursor-default select-none items-center gap-2 rounded px-2 text-base outline-none data-[highlighted]:bg-accent data-[state=open]:bg-accent data-[disabled]:opacity-50',
        className,
      )}
      {...props}
    >
      {children}
      <span className="ml-auto text-xs">›</span>
    </ContextPrimitive.SubTrigger>
  );
}

export function ContextMenuSubContent({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof ContextPrimitive.SubContent>) {
  return (
    <ContextPrimitive.Portal>
      <ContextPrimitive.SubContent
        className={cn('z-[60] min-w-[10rem] rounded-md border bg-popover p-1 shadow-md', className)}
        {...props}
      />
    </ContextPrimitive.Portal>
  );
}
