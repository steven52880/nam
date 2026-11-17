import * as DropdownPrimitive from '@radix-ui/react-dropdown-menu';
import type { ComponentPropsWithoutRef } from 'react';
import { cn } from '@/lib/utils';

export const DropdownMenu = DropdownPrimitive.Root;
export const DropdownMenuTrigger = DropdownPrimitive.Trigger;

export function DropdownMenuContent({
  className,
  sideOffset = 4,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownPrimitive.Content>) {
  return (
    <DropdownPrimitive.Portal>
      <DropdownPrimitive.Content
        sideOffset={sideOffset}
        className={cn('z-[60] min-w-[10rem] rounded-md border bg-popover p-1 shadow-md', className)}
        {...props}
      />
    </DropdownPrimitive.Portal>
  );
}

export function DropdownMenuItem({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownPrimitive.Item>) {
  return (
    <DropdownPrimitive.Item
      className={cn(
        'flex h-7 cursor-default select-none items-center gap-2 rounded px-2 text-base outline-none data-[highlighted]:bg-accent data-[disabled]:opacity-50',
        className,
      )}
      {...props}
    />
  );
}

export function DropdownMenuCheckboxItem({
  className,
  children,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownPrimitive.CheckboxItem>) {
  return (
    <DropdownPrimitive.CheckboxItem
      className={cn(
        'relative flex h-7 cursor-default select-none items-center rounded pl-6 pr-2 text-base outline-none data-[highlighted]:bg-accent',
        className,
      )}
      {...props}
    >
      <span className="absolute left-2 text-xs">
        <DropdownPrimitive.ItemIndicator>✓</DropdownPrimitive.ItemIndicator>
      </span>
      {children}
    </DropdownPrimitive.CheckboxItem>
  );
}

export function DropdownMenuLabel({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownPrimitive.Label>) {
  return <DropdownPrimitive.Label className={cn('px-2 py-1 text-sm text-muted-foreground', className)} {...props} />;
}

export const DropdownMenuSub = DropdownPrimitive.Sub;

export function DropdownMenuSubTrigger({
  className,
  children,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownPrimitive.SubTrigger>) {
  return (
    <DropdownPrimitive.SubTrigger
      className={cn(
        'flex h-7 cursor-default select-none items-center gap-2 rounded px-2 text-base outline-none data-[highlighted]:bg-accent data-[state=open]:bg-accent',
        className,
      )}
      {...props}
    >
      {children}
      <span className="ml-auto text-xs">›</span>
    </DropdownPrimitive.SubTrigger>
  );
}

export function DropdownMenuSubContent({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownPrimitive.SubContent>) {
  return (
    <DropdownPrimitive.Portal>
      <DropdownPrimitive.SubContent
        className={cn('z-[60] min-w-[10rem] rounded-md border bg-popover p-1 shadow-md', className)}
        {...props}
      />
    </DropdownPrimitive.Portal>
  );
}

export function DropdownMenuSeparator({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof DropdownPrimitive.Separator>) {
  return <DropdownPrimitive.Separator className={cn('-mx-1 my-1 h-px bg-border', className)} {...props} />;
}
