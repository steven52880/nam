import { keepPreviousData, useQueries, useQuery } from '@tanstack/react-query';
import { parseUsdCnyRate, USD_CNY_RATE_KEY } from '@/lib/usdRate';
import { api } from './endpoints';
import { keys } from './keys';
import type {
  AuditQuery,
  PriceQuery,
  StatsDim,
  StatsWindow,
  UpstreamGroup,
} from './types';

/** 全站唯一能写 useQuery / useQueries 的文件：每个接口只在这里写一次，options 固定在 hook 里。 */

interface TimeSpan {
  from: number;
  to: number;
  bucket: 'hour' | 'day';
}

const FIVE_MINUTES = 5 * 60 * 1000;

// ───────── 会话与全局 ─────────
export function useSessionQuery() {
  return useQuery({ queryKey: keys.session(), queryFn: api.session, retry: false, staleTime: Infinity });
}

export function useBackend() {
  return useQuery({ queryKey: keys.backend(), queryFn: api.backend, refetchInterval: 30000 });
}

export function useSettings() {
  return useQuery({ queryKey: keys.settings(), queryFn: api.settings });
}

/** 美元参考汇率（元/$）：设置项 usd_cny_rate；还没取到或不合法时用默认 7.2。返回数字，引用稳定。 */
export function useUsdCnyRate(): number {
  const settings = useSettings();
  return parseUsdCnyRate(settings.data?.[USD_CNY_RATE_KEY]);
}

export function useJobs() {
  return useQuery({ queryKey: keys.jobs(), queryFn: api.jobs, refetchInterval: 10000 });
}

// ───────── 上游 ─────────
export function useUpstreams() {
  return useQuery({ queryKey: keys.upstreams.list(), queryFn: api.upstreams });
}

export function useUpstreamTokens(id: number | null) {
  return useQuery({
    queryKey: keys.upstreams.tokens(id ?? 0),
    queryFn: () => api.upstreamTokens(id ?? 0),
    enabled: id !== null,
  });
}

export function useUpstreamGroups(id: number | null) {
  return useQuery({
    queryKey: keys.upstreams.groups(id ?? 0),
    queryFn: () => api.upstreamGroups(id ?? 0),
    enabled: id !== null,
  });
}

/** 上游的模型与价格（api.upstreamModels）。 */
export function useUpstreamModelPrices(id: number | null) {
  return useQuery({
    queryKey: keys.upstreams.modelPrices(id ?? 0),
    queryFn: () => api.upstreamModels(id ?? 0),
    enabled: id !== null,
  });
}

/** 每个令牌一条：从上游拉取该令牌可用的模型。 */
export function useTokenModelsFetch(upstreamId: number, tokenIds: readonly number[]) {
  return useQueries({
    queries: tokenIds.map((id) => ({
      queryKey: keys.upstreams.tokenModelsFetch(upstreamId, id),
      queryFn: async () => (await api.fetchUpstreamModels(upstreamId, id)).models,
      staleTime: FIVE_MINUTES,
      retry: false,
    })),
  });
}

// ───────── 厂商 ─────────
export function useVendors() {
  return useQuery({ queryKey: keys.vendors.list(), queryFn: api.vendors });
}

export function useVendorMatch(models: readonly string[]) {
  return useQuery({
    queryKey: keys.vendors.match(models),
    queryFn: () => api.matchVendors([...models]),
    enabled: models.length > 0,
    placeholderData: keepPreviousData,
  });
}

// ───────── 渠道 ─────────
export function useChannels() {
  return useQuery({ queryKey: keys.channels.list(), queryFn: api.channels });
}

export function useChannelPlan(open: boolean) {
  return useQuery({
    queryKey: keys.channels.plan(),
    queryFn: () => api.plan(),
    enabled: open,
    gcTime: 0,
  });
}

export function useChannelDrift(id: number | null) {
  return useQuery({
    queryKey: keys.channels.drift(id ?? 0),
    queryFn: () => api.drift(id ?? 0),
    enabled: id !== null,
  });
}

export function useChannelModels(id: number | null) {
  return useQuery({
    queryKey: keys.channels.models(id ?? 0),
    queryFn: () => api.channelModels(id ?? 0),
    enabled: id !== null && id > 0,
  });
}

/** 渠道统计。model 为 null 时是全部模型的汇总，否则只统计该模型。 */
export function useChannelStats(window: StatsWindow, model: string | null = null) {
  return useQuery({
    queryKey: keys.stats.channels(window, model),
    queryFn: () => api.statsChannels(window, model),
    refetchInterval: 60_000,
  });
}

/** 某个模型在各渠道上的成本；model 为 null 时不请求。随渠道数据一起失效（keys.channels.all）。 */
export function useChannelCosts(model: string | null) {
  return useQuery({
    queryKey: keys.channels.costs(model ?? ''),
    queryFn: () => api.channelCosts(model ?? ''),
    enabled: model !== null,
  });
}

export function useChannelModelStats(id: number | null, window: StatsWindow) {
  return useQuery({
    queryKey: keys.stats.channelModels(id ?? 0, window),
    queryFn: () => api.statsChannelModels(id ?? 0, window),
    enabled: id !== null && id > 0,
    refetchInterval: 60_000,
  });
}

export function useLatestCacheTests() {
  return useQuery({
    queryKey: keys.channels.latestCacheTests(),
    queryFn: api.latestCacheTests,
    refetchInterval: (q) => (q.state.data?.items.some((t) => t.status === 'running') ? 3000 : false),
  });
}

export function useCacheTest(testId: number | null) {
  return useQuery({
    queryKey: keys.channels.cacheTest(testId ?? 0),
    queryFn: () => api.cacheTest(testId ?? 0),
    enabled: testId !== null,
    refetchInterval: (q) => (q.state.data?.status === 'running' ? 2000 : false),
    gcTime: 0,
  });
}

// ───────── 价格 ─────────
export function usePriceList(q: PriceQuery) {
  return useQuery({ queryKey: keys.prices.list(q), queryFn: () => api.prices(q) });
}

/** 供侧边栏角标使用：只取全部列表里的统计数。 */
export function usePriceCounts() {
  return useQuery({
    queryKey: keys.prices.counts(),
    queryFn: () => api.prices({}),
    select: (data) => data.counts,
    staleTime: 60_000,
  });
}

export function usePriceDetail(model: string | null) {
  return useQuery({
    queryKey: keys.prices.detail(model ?? ''),
    queryFn: () => api.priceDetail(model ?? ''),
    enabled: model !== null,
  });
}

// ───────── 统计 ─────────
export function useStatsOverview(range: TimeSpan | null, dim: StatsDim) {
  return useQuery({
    queryKey: keys.stats.overview(range?.from ?? 0, range?.to ?? 0, dim),
    queryFn: () => api.statsOverview({ from: range?.from ?? 0, to: range?.to ?? 0, dim }),
    enabled: range !== null,
  });
}

export function useStatsSeries(range: TimeSpan | null) {
  return useQuery({
    queryKey: keys.stats.series(range?.from ?? 0, range?.to ?? 0, range?.bucket ?? 'day'),
    queryFn: () =>
      api.statsSeries({ from: range?.from ?? 0, to: range?.to ?? 0, bucket: range?.bucket ?? 'day' }),
    enabled: range !== null,
  });
}

/** 单个渠道的逐小时序列（统计页抽屉）。 */
export function useChannelSeries(key: string, range: { from: number; to: number }) {
  return useQuery({
    queryKey: keys.stats.channelSeries(key, range.from, range.to),
    queryFn: () => api.statsSeries({ from: range.from, to: range.to, dim: 'channel', key, bucket: 'hour' }),
  });
}

export function useStatsStatus() {
  return useQuery({ queryKey: keys.stats.status(), queryFn: api.statsStatus, refetchInterval: 30_000 });
}

// ───────── 审计 ─────────
export function useAudit(q: AuditQuery) {
  return useQuery({
    queryKey: keys.audit.list(q),
    queryFn: () => api.audit(q),
    placeholderData: (prev) => prev,
  });
}

export function useAuditRecent() {
  return useQuery({ queryKey: keys.audit.recent(), queryFn: () => api.audit({ limit: 5 }) });
}

// ───────── 小工具 ─────────
/** 按分组名查倍率；找不到返回 null。 */
export function groupRatioOf(groups: readonly UpstreamGroup[] | undefined, name: string): number | null {
  return groups?.find((g) => g.name === name)?.ratio ?? null;
}
