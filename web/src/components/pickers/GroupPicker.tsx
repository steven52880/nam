import { useMemo } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { useUpstreamGroups, useUsdCnyRate } from '@/api/queries';
import type { Upstream, UpstreamGroup } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Combobox, type ComboboxColumn, type ComboboxItem } from '@/components/ui/combobox';
import { useActionMutation } from '@/hooks/useActionMutation';
import { formatNumber, formatRatioPercent } from '@/lib/format';
import { channelPrice } from '@/lib/usdRate';
import type { EntityPickerProps } from './types';

type GroupItem = ComboboxItem & { raw: UpstreamGroup };

const NO_GROUP = '还没有分组，可以点下面同步';

export function GroupPicker({
  upstream,
  value,
  onChange,
  variant,
  disabled,
  placeholder,
  title,
  className,
  fallbackLabel,
}: EntityPickerProps<string> & { upstream: Upstream }) {
  const groups = useUpstreamGroups(upstream.id);
  const rate = upstream.currency_rate;
  const usdRate = useUsdCnyRate();

  const items = useMemo<GroupItem[]>(
    () => (groups.data ?? []).map((g) => ({ value: g.name, label: g.name, raw: g })),
    [groups.data],
  );
  const columns = useMemo<ComboboxColumn<GroupItem>[]>(
    () => [
      { id: 'ratio', header: '渠道倍率', width: 5, render: (it) => formatNumber(it.raw.ratio, 3) },
      {
        id: 'price_ratio',
        header: '上游渠道售价',
        width: 6,
        render: (it) => formatRatioPercent(channelPrice(it.raw.ratio, rate, usdRate)),
      },
    ],
    [rate, usdRate],
  );

  const sync = useActionMutation({
    mutationFn: () => api.syncUpstream(upstream.id, ['groups']),
    invalidate: invalidate.upstream,
    onSuccess: (results) => {
      const bad = results.find((r) => !r.ok);
      if (bad) toast.error(`同步分组失败：${bad.error ?? ''}`);
      else toast.success('已同步分组');
    },
  });

  const footer = (
    <Button type="button" variant="ghost" size="sm" disabled={sync.isPending} onClick={() => sync.mutate()}>
      {sync.isPending ? '同步中…' : '同步分组'}
    </Button>
  );

  return (
    <Combobox
      items={items}
      value={value}
      onChange={(v) => onChange(v ?? '')}
      columns={columns}
      clearable
      variant={variant}
      disabled={disabled}
      placeholder={placeholder}
      title={title}
      className={className}
      valueLabel={fallbackLabel}
      loading={groups.isLoading}
      allowCustom
      emptyText={items.length === 0 ? NO_GROUP : undefined}
      footer={footer}
    />
  );
}
