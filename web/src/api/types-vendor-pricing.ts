/* ---------- M5：厂商定价 ---------- */

/** 固定 = 手填售价；跟随 = 参考成本 × 加价比例。 */
export type VendorPricingMode = 'fixed' | 'follow';

/** 已同步 / 待写入 / 待确认 / 漂移 / 无可用渠道 / 未设置。 */
export type VendorPricingState = 'synced' | 'to_write' | 'pending' | 'drift' | 'no_channel' | 'unset';

export interface VendorRefChannel {
  id: number;
  name: string;
  weight: number;
  /** 上游渠道售价（原厂价的倍数）。 */
  channel_cost: number | null;
}

/** 参考成本：最高优先级一层启用渠道的上游渠道售价，按权重加权。 */
export interface VendorReference {
  /** 原厂价的倍数。 */
  cost: number;
  priority: number;
  channels: VendorRefChannel[];
}

export interface VendorPricingItem {
  vendor_id: number;
  vendor: string;
  /** NewAPI 里的分组名。 */
  group: string;
  order: number;
  mode: VendorPricingMode;
  /** 加价比例，只在跟随模式有意义。 */
  markup: number | null;
  /** 售价（原厂价的倍数），只在固定模式有意义。 */
  sell_price: number | null;
  reference: VendorReference | null;
  target_price: number | null;
  /** 厂商分组倍率（目标）。 */
  target_ratio: number | null;
  /** NewAPI 当前倍率；null = NewAPI 里还没有这个分组。 */
  newapi_ratio: number | null;
  last_written_price: number | null;
  last_written_ratio: number | null;
  /** Unix 秒；接管得到、还没写过时为 null。 */
  last_written_at: number | null;
  pending_price: number | null;
  /** |目标售价 − 上次写入售价| ÷ 上次写入售价。 */
  change: number | null;
  /** 变化小于最小变动：自动写入会跳过。 */
  small_change: boolean;
  state: VendorPricingState;
  revenue_7d: number | null;
  cost_7d: number | null;
  margin_7d: number | null;
}

export interface VendorGroupsPlan {
  auto_groups: { current: string[]; target: string[] };
  user_usable_groups: { current: Record<string, string>; target: Record<string, string> };
  changed: boolean;
}

export interface VendorPricingOverview {
  site_rate: number;
  usd_rate: number;
  auto_write: boolean;
  /** 变化保护阈值（0.5 = 50%）。 */
  protect_threshold: number;
  /** 最小变动（0.02 = 2%）。 */
  min_change: number;
  items: VendorPricingItem[];
  groups: VendorGroupsPlan;
  /** 非空表示 NewAPI 选项解析失败，此时不能写入。 */
  error: string;
}

export interface VendorPricingUpdate {
  mode?: VendorPricingMode;
  /** 固定模式的售价（原厂价的倍数）。 */
  sell_price?: number;
  markup?: number;
  /** 直接指定厂商分组倍率（3 位小数）；后端会反算成售价。不要和 sell_price 一起传。 */
  group_ratio?: number;
}

export interface VendorWriteRequest {
  vendor_ids?: number[];
  include_groups?: boolean;
  force?: boolean;
}

export interface VendorWriteChange {
  vendor_id: number;
  vendor: string;
  group: string;
  before_price: number | null;
  after_price: number | null;
  before_ratio: number | null;
  after_ratio: number | null;
}

export interface VendorWriteSkipped {
  vendor_id: number;
  vendor: string;
  state: VendorPricingState;
  reason: string;
}

export interface VendorOptionDiff {
  key: string;
  before: string;
  after: string;
}

export interface VendorPreview {
  vendors: VendorWriteChange[];
  skipped: VendorWriteSkipped[];
  options: VendorOptionDiff[];
}

export interface VendorApplyResult {
  written: string[];
  vendors: VendorWriteChange[];
  skipped: VendorWriteSkipped[];
}
