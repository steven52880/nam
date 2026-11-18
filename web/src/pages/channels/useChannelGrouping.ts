import { useCallback, useMemo } from 'react';
import { useVendors } from '@/api/queries';
import type { ChannelRow } from '@/api/types';
import { mergeVisibleOrder } from '@/components/table';
import type { GroupMode } from './constants';
import { buildGroupLookups, groupKeyOf, groupLabelOf, sortGroupKeys } from './grouping';

const NO_ORDER: string[] = [];

/**
 * 渠道表的分组：组键、组名、组顺序、拖动写回。
 * 传给表格的函数引用都稳定；rows 必须是全部渠道（未筛选），这样拖动时能拿到完整的组列表。
 */
export function useChannelGrouping(
  rows: ChannelRow[],
  mode: GroupMode,
  /** 当前模式保存的组顺序（上游 / 标签）。 */
  savedOrder: string[] | undefined,
  /** 写回当前模式的完整组顺序；必须引用稳定。 */
  saveOrder: (mode: 'upstream' | 'tag', order: string[]) => void,
) {
  const vendors = useVendors();
  const vendorOrder = useMemo(
    () => [...(vendors.data ?? [])].sort((a, b) => a.order - b.order || a.name.localeCompare(b.name)).map((v) => v.name),
    [vendors.data],
  );
  const lookups = useMemo(() => buildGroupLookups(rows), [rows]);
  const groupBy = useMemo(() => groupKeyOf(mode), [mode]);
  const groupLabel = useCallback((key: string) => groupLabelOf(mode, key, lookups), [mode, lookups]);
  const saved = savedOrder ?? NO_ORDER;

  const sortGroups = useCallback(
    (keys: string[]) => sortGroupKeys(mode, keys, { vendorOrder, savedOrder: saved, labelOf: groupLabel }),
    [mode, vendorOrder, saved, groupLabel],
  );

  // 厂商顺序在设置页改，按厂商分组时不能拖动
  const onGroupReorder = useMemo(() => {
    if (mode !== 'upstream' && mode !== 'tag') return undefined;
    const keyOf = groupKeyOf(mode);
    if (!keyOf) return undefined;
    return (visible: string[]) => {
      const fullKeys = [...new Set(rows.map(keyOf))];
      const fullOrder = sortGroupKeys(mode, fullKeys, { vendorOrder, savedOrder: saved, labelOf: groupLabel });
      saveOrder(mode, mergeVisibleOrder(fullOrder, visible));
    };
  }, [mode, rows, vendorOrder, saved, groupLabel, saveOrder]);

  return { groupBy, groupLabel, sortGroups, onGroupReorder };
}
