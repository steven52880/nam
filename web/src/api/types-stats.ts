/** M4 统计接口的类型，契约见 .omo/plans/m4-stats.md「JSON 契约」。比率都是 0–1 的小数。 */

/** latest，或 <正整数>h / <正整数>d（上限 186 天）；校验见 lib/statsWindow.ts。 */
export type StatsWindow = 'latest' | `${number}h` | `${number}d`;
export type StatsDim = 'channel' | 'model' | 'upstream' | 'vendor';
export type StatsBucket = 'hour' | 'day';
export type FailClass = '429' | '5xx' | 'timeout' | '401' | 'balance' | 'other';
export type PriceTier = 'cheap' | 'mid' | 'expensive';
export type QualityTier = 'good' | 'mid' | 'bad' | 'insufficient';

export interface StatsMetrics {
  requests: number;
  success: number;
  fail: number;
  success_rate: number | null;
  /** 成功率的 Wilson 下界（z = 1.96）。 */
  wilson: number | null;
  fail_by_class: Partial<Record<FailClass, number>>;
  /** 首字延迟（毫秒）。 */
  frt_p50: number | null;
  frt_p95: number | null;
  /** 吐字速度（token / 秒）。 */
  speed_p50: number | null;
  speed_p95: number | null;
  /** true = NewAPI 未打补丁 P1，按整秒估算。 */
  speed_estimated: boolean;
  cache_hit: number | null;
  /** 非缓存输入 token。 */
  prompt_tokens: number;
  completion_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  cost_cny: number;
  revenue_cny: number;
  /** 能算出成本的请求数。 */
  priced_requests: number;
  /**
   * 实际负载成本（原「实测成本」，有效口径）：渠道真实请求的实际花费 ÷ 同样请求不算缓存、按原厂价计的费用
   * （统一成人民币后相除）。后端已除以美元参考汇率，值是「原厂价的倍数」（0.024 = 2.4%），前端不再除；缓存命中越高越低。
   */
  effective_cost_price: number | null;
  /**
   * 实际计费倍率（原「定价口径成本价」）：实际花费 ÷ 算上缓存优惠的原厂价，值是原厂价的倍数。
   * 正常应约等于上游渠道售价，明显偏高说明上游多扣了。
   */
  measured_cost_price: number | null;
  margin: number | null;
}

export interface ChannelStatsItem extends StatsMetrics {
  channel_id: number;
  /**
   * 标准负载成本（原「标准负载成本价」）：用同厂商所有渠道近 7 天的平均 token 比例，按渠道配置的倍率、汇率、
   * 缓存价理论计算的 实际花费 ÷ 不算缓存的原厂价；值是原厂价的倍数。价格档按它划分。
   */
  std_cost_price: number | null;
  price_tier: PriceTier | null;
  quality_tier: QualityTier;
}

export interface ChannelStatsList {
  window: StatsWindow;
  /** 请求时带的模型；空字符串 = 全部模型的汇总。 */
  model: string;
  generated_at: number;
  patch_p1: boolean;
  error_log_hint: boolean;
  items: ChannelStatsItem[];
}

export interface ModelStatsItem extends StatsMetrics {
  model: string;
}

export interface ModelStatsList {
  items: ModelStatsItem[];
}

export interface StatsRow extends StatsMetrics {
  key: string;
  name: string;
}

export interface ScatterPoint {
  channel_id: number;
  name: string;
  vendor: string;
  /** 标准负载成本（原厂价的倍数），含义同 ChannelStatsItem.std_cost_price。 */
  std_cost_price: number | null;
  wilson: number | null;
  requests: number;
}

export interface StatsOverview {
  from: number;
  to: number;
  dim: StatsDim;
  patch_p1: boolean;
  error_log_hint: boolean;
  totals: StatsMetrics;
  rows: StatsRow[];
  error_classes: Partial<Record<FailClass, number>>;
  scatter: ScatterPoint[];
}

export interface SeriesPoint extends StatsMetrics {
  t: number;
}

export interface StatsSeries {
  bucket: StatsBucket;
  points: SeriesPoint[];
}

export interface StatsStatus {
  cursor_id: number;
  cursor_at: number;
  rows: number;
  oldest_hour: number | null;
  backfill_days: number;
  patch_p1: boolean;
  error_log_hint: boolean;
  last_pull_at: number;
  last_pull_message: string;
}

export interface OverviewQuery {
  from: number;
  to: number;
  dim: StatsDim;
}

export interface SeriesQuery {
  from: number;
  to: number;
  dim?: StatsDim;
  key?: string;
  bucket: StatsBucket;
}

