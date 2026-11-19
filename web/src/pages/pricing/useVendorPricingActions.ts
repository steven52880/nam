import { useCallback } from 'react';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import type { VendorApplyResult, VendorPricingItem, VendorPricingMode, VendorPricingUpdate } from '@/api/types';
import { useActionMutation } from '@/hooks/useActionMutation';

/** 写入结果的提示文字。 */
export function applyToast(r: VendorApplyResult): string {
  if (r.vendors.length > 0) return `已写入 ${r.vendors.length} 个厂商的分组倍率`;
  if (r.written.length > 0) return '已更新分组选项';
  return '没有需要写入的内容';
}

interface UpdateVars {
  row: VendorPricingItem;
  body: VendorPricingUpdate;
}

/** 厂商定价表里的提交：编辑（只写 nam）、撤销修改、确认。 */
export function useVendorPricingActions() {
  const update = useActionMutation<UpdateVars, VendorPricingItem>({
    mutationFn: ({ row, body }) => api.updateVendorPricing(row.vendor_id, body),
    invalidate: invalidate.vendorPricing,
    success: (item, { body }) => {
      if (body.discard_draft) return '已撤销修改';
      return item.state === 'to_write' ? '已保存为待写入，点「预览写入」后生效' : '已保存';
    },
  });
  const confirm = useActionMutation<number, VendorApplyResult>({
    mutationFn: (id) => api.confirmVendorPricing(id),
    invalidate: invalidate.vendorPricing,
    success: applyToast,
  });

  const updateMutate = update.mutate;
  const setMode = useCallback(
    (row: VendorPricingItem, mode: VendorPricingMode) => {
      if (mode === row.mode) return;
      // 切到跟随时没有加价比例就给个 1，免得算不出目标；切到固定时没有手填售价就沿用当前目标售价
      const body: VendorPricingUpdate = { mode };
      if (mode === 'follow' && row.markup === null) body.markup = 1;
      if (mode === 'fixed' && row.sell_price === null && row.target_price !== null) body.sell_price = row.target_price;
      updateMutate({ row, body });
    },
    [updateMutate],
  );
  const setMarkup = useCallback(
    (row: VendorPricingItem, markup: number) => updateMutate({ row, body: { markup } }),
    [updateMutate],
  );
  const setSellPrice = useCallback(
    (row: VendorPricingItem, sell_price: number) => updateMutate({ row, body: { sell_price } }),
    [updateMutate],
  );
  const setGroupRatio = useCallback(
    (row: VendorPricingItem, group_ratio: number) => updateMutate({ row, body: { group_ratio } }),
    [updateMutate],
  );
  const discardDraft = useCallback(
    (row: VendorPricingItem) => updateMutate({ row, body: { discard_draft: true } }),
    [updateMutate],
  );
  return { update, confirm, setMode, setMarkup, setSellPrice, setGroupRatio, discardDraft };
}

/** 页头「立即计算」：触发定价任务并等它跑完，提示里带任务摘要。 */
export function useCalculatePricing() {
  return useActionMutation<void, string>({
    mutationFn: async () => {
      await api.runJobAndWait('pricing');
      const job = (await api.jobs()).find((j) => j.name === 'pricing');
      if (job?.last_result === 'fail') throw new Error(job.last_message || '定价计算失败');
      return job?.last_message ?? '';
    },
    invalidate: invalidate.vendorPricing,
    success: (message) => (message ? `定价计算完成：${message}` : '定价计算完成'),
  });
}
