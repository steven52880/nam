import { useMemo } from 'react';
import { useVendorMatches } from '@/components/model-picker/useModelGroups';
import { Combobox, type ComboboxColumn, type ComboboxItem } from '@/components/ui/combobox';

export interface ModelOption {
  model: string;
  channelCount: number;
}

type ModelItem = ComboboxItem & { channelCount: number };

const UNOWNED_LABEL = '未归属';

const COLUMNS: ComboboxColumn<ModelItem>[] = [
  { id: 'channels', header: '渠道数', width: 4, render: (it) => it.channelCount },
];

interface ModelComboboxProps {
  value: string | null;
  onChange: (value: string | null) => void;
  options: ModelOption[];
  variant?: 'field' | 'toolbar';
  className?: string;
}

export function ModelCombobox({ value, onChange, options, variant, className }: ModelComboboxProps) {
  const models = useMemo(() => options.map((o) => o.model), [options]);
  const { matches } = useVendorMatches(models);

  const items = useMemo<ModelItem[]>(
    () =>
      options.map((o) => ({
        value: o.model,
        label: o.model,
        group: matches.get(o.model)?.vendor || UNOWNED_LABEL,
        channelCount: o.channelCount,
      })),
    [options, matches],
  );

  const missing = value !== null && value !== '' && !options.some((o) => o.model === value);

  return (
    <Combobox
      items={items}
      value={value}
      onChange={(v) => onChange(v)}
      columns={COLUMNS}
      variant={variant}
      className={className}
      clearable
      notice={missing ? '没有渠道包含该模型' : undefined}
    />
  );
}
