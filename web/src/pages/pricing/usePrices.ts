import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/endpoints';
import type { PriceQuery } from '@/api/types';
import { PRICES_KEY } from './constants';

export function usePriceList(query: PriceQuery) {
  return useQuery({ queryKey: [...PRICES_KEY, query], queryFn: () => api.prices(query) });
}

/** 供侧边栏角标使用：只取全部列表里的统计数。 */
export function usePriceCounts() {
  return useQuery({
    queryKey: [...PRICES_KEY, 'counts'],
    queryFn: () => api.prices({}),
    select: (data) => data.counts,
    staleTime: 60_000,
  });
}
