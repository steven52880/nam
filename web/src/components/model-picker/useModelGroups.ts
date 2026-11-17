import { useMemo } from 'react';
import { useVendorMatch } from '@/api/queries';
import type { VendorMatchBy, VendorMatchItem } from '@/api/types';

export const UNOWNED_KEY = '';
export const OTHER_KEY = '\u0000other';

export interface PickerModel {
  model: string;
  by: VendorMatchBy;
  prefix: string;
}

export interface ModelGroup {
  /** 厂商名；UNOWNED_KEY = 未归属，OTHER_KEY = 其他厂商。 */
  key: string;
  label: string;
  models: PickerModel[];
}

const collator = new Intl.Collator('zh-CN');

/** 把模型按厂商分组：厂商组按名称排序，「其他厂商」「未归属」固定放最后。 */
export function groupModels(
  models: readonly string[],
  matches: ReadonlyMap<string, VendorMatchItem>,
  vendorFilter?: readonly string[],
): ModelGroup[] {
  const filter = vendorFilter && vendorFilter.length > 0 ? new Set(vendorFilter) : null;
  const buckets = new Map<string, PickerModel[]>();
  for (const model of models) {
    const m = matches.get(model);
    let key = m?.vendor ?? UNOWNED_KEY;
    if (key !== UNOWNED_KEY && filter && !filter.has(key)) key = OTHER_KEY;
    const item: PickerModel = { model, by: m?.by ?? '', prefix: m?.prefix ?? '' };
    const list = buckets.get(key);
    if (list) list.push(item);
    else buckets.set(key, [item]);
  }
  const rank = (key: string): number => (key === UNOWNED_KEY ? 2 : key === OTHER_KEY ? 1 : 0);
  return [...buckets.entries()]
    .map(([key, list]) => ({
      key,
      label: key === UNOWNED_KEY ? '未归属' : key === OTHER_KEY ? '其他厂商' : key,
      models: list.sort((a, b) => collator.compare(a.model, b.model)),
    }))
    .sort((a, b) => rank(a.key) - rank(b.key) || collator.compare(a.label, b.label));
}

/** 查询后端厂商匹配结果，返回 model → 匹配项 的映射。 */
export function useVendorMatches(models: readonly string[]) {
  const sorted = useMemo(() => [...new Set(models)].sort(), [models]);
  const query = useVendorMatch(sorted);
  const matches = useMemo(() => {
    const map = new Map<string, VendorMatchItem>();
    for (const item of query.data?.items ?? []) map.set(item.model, item);
    return map;
  }, [query.data]);
  return { matches, isLoading: query.isLoading, isError: query.isError };
}
