import { useMemo } from 'react';
import { useUpstreamTokens } from '@/api/queries';
import { Combobox, type ComboboxItem } from '@/components/ui/combobox';
import type { EntityPickerProps } from './types';

export function TokenPicker({
  upstreamId,
  value,
  onChange,
  variant,
  disabled,
  placeholder,
  title,
  className,
  fallbackLabel,
}: EntityPickerProps<number> & { upstreamId: number | null }) {
  const tokens = useUpstreamTokens(upstreamId);
  const items = useMemo<ComboboxItem[]>(
    () =>
      (tokens.data ?? []).map((t) => ({
        value: String(t.id),
        label: `${t.name}${t.manual ? '（手动）' : ''}`,
        keywords: [t.name],
      })),
    [tokens.data],
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
      loading={tokens.isLoading}
    />
  );
}
