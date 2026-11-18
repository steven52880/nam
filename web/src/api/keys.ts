import type { AuditQuery, PriceQuery, StatsBucket, StatsDim, StatsWindow } from './types';

/** 查询键工厂：全站唯一能写 queryKey 数组的地方。同一类数据共用前缀，失效前缀即可刷新整类。 */
export const keys = {
  session: () => ['session'] as const,
  backend: () => ['backend'] as const,
  settings: () => ['settings'] as const,
  jobs: () => ['jobs'] as const,
  upstreams: {
    all: () => ['upstreams'] as const,
    list: () => ['upstreams', 'list'] as const,
    tokens: (id: number) => ['upstreams', id, 'tokens'] as const,
    groups: (id: number) => ['upstreams', id, 'groups'] as const,
    /** 上游的模型与价格（api.upstreamModels）。 */
    modelPrices: (id: number) => ['upstreams', id, 'model-prices'] as const,
    tokenModelsFetch: (id: number, tokenId: number) => ['upstreams', id, 'token-models-fetch', tokenId] as const,
  },
  vendors: {
    all: () => ['vendors'] as const,
    list: () => ['vendors', 'list'] as const,
    match: (models: readonly string[]) => ['vendors', 'match', [...models].sort()] as const,
  },
  channels: {
    all: () => ['channels'] as const,
    list: () => ['channels', 'list'] as const,
    plan: () => ['channels', 'plan'] as const,
    drift: (id: number) => ['channels', 'drift', id] as const,
    models: (id: number) => ['channels', 'models', id] as const,
    costs: (model: string) => ['channels', 'costs', model] as const,
    latestCacheTests: () => ['channels', 'cache-tests', 'latest'] as const,
    cacheTest: (testId: number) => ['channels', 'cache-test', testId] as const,
  },
  prices: {
    all: () => ['prices'] as const,
    list: (q: PriceQuery) => ['prices', 'list', q.status, q.vendor, q.q] as const,
    counts: () => ['prices', 'counts'] as const,
    detail: (model: string) => ['prices', 'detail', model] as const,
  },
  vendorPricing: {
    all: () => ['vendor-pricing'] as const,
    overview: () => ['vendor-pricing', 'overview'] as const,
    preview: (includeGroups: boolean) => ['vendor-pricing', 'preview', includeGroups] as const,
  },
  stats: {
    all: () => ['stats'] as const,
    channels: (window: StatsWindow, model: string | null) => ['stats', 'channels', window, model ?? ''] as const,
    channelModels: (id: number, window: StatsWindow) => ['stats', 'channel-models', id, window] as const,
    overview: (from: number, to: number, dim: StatsDim) => ['stats', 'overview', from, to, dim] as const,
    series: (from: number, to: number, bucket: StatsBucket) => ['stats', 'series', from, to, bucket] as const,
    channelSeries: (key: string, from: number, to: number) => ['stats', 'channel-series', key, from, to] as const,
    status: () => ['stats', 'status'] as const,
  },
  audit: {
    all: () => ['audit'] as const,
    list: (q: AuditQuery) => ['audit', 'list', q.kind, q.trigger, q.result, q.q, q.from, q.to, q.limit, q.offset] as const,
    recent: () => ['audit', 'recent'] as const,
  },
};
