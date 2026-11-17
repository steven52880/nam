import type { QueryClient } from '@tanstack/react-query';
import { keys } from './keys';

/** “改了什么 → 刷新什么”只写在这里；提交成功后调用对应函数，页面不自己拼失效列表。 */
const all = async (...tasks: Array<Promise<void>>): Promise<void> => {
  await Promise.all(tasks);
};

export const invalidate = {
  /** 上游：新增、编辑、删除、同步、换凭证、改汇率。 */
  upstream: (qc: QueryClient) =>
    all(qc.invalidateQueries({ queryKey: keys.upstreams.all() }), qc.invalidateQueries({ queryKey: keys.channels.all() })),
  /** 令牌：新增、删除、改模型。 */
  tokens: (qc: QueryClient, upstreamId: number) =>
    all(
      qc.invalidateQueries({ queryKey: keys.upstreams.tokens(upstreamId) }),
      qc.invalidateQueries({ queryKey: keys.upstreams.list() }),
      qc.invalidateQueries({ queryKey: keys.channels.all() }),
    ),
  /** 厂商：增删改、排序。 */
  vendors: (qc: QueryClient) =>
    all(
      qc.invalidateQueries({ queryKey: keys.vendors.all() }),
      qc.invalidateQueries({ queryKey: keys.channels.all() }),
      qc.invalidateQueries({ queryKey: keys.prices.all() }),
    ),
  /** 渠道草稿、应用、漂移处理、接管。 */
  channels: (qc: QueryClient) =>
    all(
      qc.invalidateQueries({ queryKey: keys.channels.all() }),
      qc.invalidateQueries({ queryKey: keys.stats.all(), predicate: (q) => q.queryKey[1] === 'channels' }),
    ),
  /** 缓存测试开始、结束、取消。 */
  cacheTests: (qc: QueryClient) => qc.invalidateQueries({ queryKey: keys.channels.latestCacheTests() }),
  /** 价格覆盖、别名、模型级折扣、缓存覆盖。 */
  prices: (qc: QueryClient) =>
    all(qc.invalidateQueries({ queryKey: keys.prices.all() }), qc.invalidateQueries({ queryKey: keys.channels.all() })),
  /** 设置保存（含调度间隔）。 */
  settings: (qc: QueryClient) =>
    all(qc.invalidateQueries({ queryKey: keys.settings() }), qc.invalidateQueries({ queryKey: keys.jobs() })),
  /** 手动触发任务。 */
  jobs: (qc: QueryClient) => qc.invalidateQueries({ queryKey: keys.jobs() }),
  /** 统计重算、清空。 */
  stats: (qc: QueryClient) => qc.invalidateQueries({ queryKey: keys.stats.all() }),
};
