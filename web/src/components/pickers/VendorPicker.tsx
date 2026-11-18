import { useMemo } from 'react';
import { useVendors } from '@/api/queries';
import { Combobox, type ComboboxItem } from '@/components/ui/combobox';
import type { EntityPickerProps } from './types';

const NO_VENDOR = '还没有厂商，请先到「设置 → 厂商分组」添加';

export function VendorPicker({
  value,
  onChange,
  variant,
  disabled,
  placeholder,
  title,
  className,
  fallbackLabel,
}: EntityPickerProps<string>) {
  const vendors = useVendors();
  const items = useMemo<ComboboxItem[]>(
    () => (vendors.data ?? []).map((v) => ({ value: v.name, label: v.name })),
    [vendors.data],
  );
  const noVendor = vendors.isSuccess && items.length === 0;
  return (
    <Combobox
      items={items}
      value={value}
      onChange={(v) => {
        if (v !== null) onChange(v);
      }}
      variant={variant}
      disabled={disabled}
      placeholder={placeholder}
      title={title}
      className={className}
      valueLabel={fallbackLabel}
      loading={vendors.isLoading}
      emptyText={noVendor ? NO_VENDOR : undefined}
    />
  );
}
