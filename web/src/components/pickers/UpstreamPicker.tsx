import { useMemo } from 'react';
import { useUpstreams } from '@/api/queries';
import { Combobox, type ComboboxItem } from '@/components/ui/combobox';
import type { EntityPickerProps } from './types';

export function UpstreamPicker({
  value,
  onChange,
  variant,
  disabled,
  placeholder,
  title,
  className,
  fallbackLabel,
}: EntityPickerProps<number>) {
  const upstreams = useUpstreams();
  const items = useMemo<ComboboxItem[]>(
    () => (upstreams.data ?? []).map((u) => ({ value: String(u.id), label: u.name })),
    [upstreams.data],
  );
  return (
    <Combobox
      items={items}
      value={value === null ? null : String(value)}
      onChange={(v) => {
        if (v !== null) onChange(Number(v));
      }}
      variant={variant}
      disabled={disabled}
      placeholder={placeholder}
      title={title}
      className={className}
      valueLabel={fallbackLabel}
      loading={upstreams.isLoading}
    />
  );
}
