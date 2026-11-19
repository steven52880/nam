import type { QueryClient } from '@tanstack/react-query';
import { keys } from './keys';

/** “改了什么 → 刷新什么”只写在这里；提交成功后调用对应函数，页面不自己拼失效列表。 */
const all = async (...tasks: Array<Promise<void>>): Promise<void> => {
  await Promise.all(tasks);
};

export const invalidate = {
  /** 厂商定价：编辑、撤销修改、计算、写入、确认。写入会产生审计，一并刷新。 */
  vendorPricing: (qc: QueryClient) =>
    all(
      qc.invalidateQueries({ queryKey: keys.vendorPricing.all() }),
      qc.invalidateQueries({ queryKey: keys.audit.all() }),
      qc.invalidateQueries({ queryKey: keys.jobs() }),
    ),
  /** 上游：新增、编辑、删除、同步、换凭证、改汇率。 */
  upstream: (qc: QueryClient) =>
    all(
      qc.invalidateQueries({ queryKey: keys.upstreams.all() }),
      qc.invalidateQueries({ queryKey: keys.channels.all() }),
      qc.invalidateQueries({ queryKey: keys.vendorPricing.all() }),
    ),
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
      qc.invalidateQueries({ queryKey: keys.vendorPricing.all() }),
    ),
  /** 渠道草稿、应用、接管。 */
  channels: (qc: QueryClient) =>
    all(
      qc.invalidateQueries({ queryKey: keys.channels.all() }),
      qc.invalidateQueries({ queryKey: keys.vendorPricing.all() }),
      qc.invalidateQueries({ queryKey: keys.stats.all(), predicate: (q) => q.queryKey[1] === 'channels' }),
    ),
  /** 缓存测试开始、结束、取消。 */
  cacheTests: (qc: QueryClient) => qc.invalidateQueries({ queryKey: keys.channels.latestCacheTests() }),
  /** 价格覆盖、别名、模型级倍率、缓存覆盖。 */
  prices: (qc: QueryClient) =>
    all(qc.invalidateQueries({ queryKey: keys.prices.all() }), qc.invalidateQueries({ queryKey: keys.channels.all() })),
  /** 设置保存（含调度间隔、美元参考汇率）：参考汇率会改变渠道售价等成本数值，所以渠道和统计一并刷新。 */
  settings: (qc: QueryClient) =>
    all(
      qc.invalidateQueries({ queryKey: keys.settings() }),
      qc.invalidateQueries({ queryKey: keys.jobs() }),
      qc.invalidateQueries({ queryKey: keys.channels.all() }),
      qc.invalidateQueries({ queryKey: keys.stats.all() }),
      qc.invalidateQueries({ queryKey: keys.vendorPricing.all() }),
    ),
  /** 手动触发任务。 */
  jobs: (qc: QueryClient) => qc.invalidateQueries({ queryKey: keys.jobs() }),
  /** 统计重算、清空。 */
  stats: (qc: QueryClient) => qc.invalidateQueries({ queryKey: keys.stats.all() }),
};
