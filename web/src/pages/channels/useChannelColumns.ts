import { useMemo } from 'react';
import { useLatestCacheTests, useVendors } from '@/api/queries';
import { buildChannelColumns } from './columns';
import type { ChannelsPageState } from './useChannelsPage';

/** 渠道表的列：把页面状态里的回调、厂商分组、手动缓存测试、成本范围接到列定义上。 */
export function useChannelColumns(page: ChannelsPageState) {
  const { setModelRow, setDriftRow } = page.dialogs;
  const { setToken, setVendor } = page.actions;
  // 只依赖引用稳定的 mutate，不依赖每次状态变化都会换新的 mutation 对象
  const setUpstream = page.actions.setUpstream.mutate;
  const ranges = page.view.ranges;

  const vendors = useVendors();
  const vendorGroups = useMemo(
    () => new Map((vendors.data ?? []).map((v) => [v.name, v.group_name])),
    [vendors.data],
  );
  const cacheTestList = useLatestCacheTests();
  const cacheTests = useMemo(
    () => new Map((cacheTestList.data?.items ?? []).map((t) => [t.channel_id, t])),
    [cacheTestList.data],
  );

  return useMemo(
    () =>
      buildChannelColumns({
        onOpenModels: setModelRow,
        onOpenDrift: setDriftRow,
        onSetUpstream: (row, upstreamId) => setUpstream({ targets: [row], upstreamId }),
        onSetToken: setToken,
        onSetVendor: setVendor,
        vendorGroups,
        cacheTests,
        ranges,
      }),
    [setModelRow, setDriftRow, setUpstream, setToken, setVendor, vendorGroups, cacheTests, ranges],
  );
}
