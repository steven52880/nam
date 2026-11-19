import type { JobStatus } from './types';
import type { DiscountSource } from './types-channels';

/* ---------- M3：价格与模型明细 ---------- */

/** 原厂价，单位$/M token（按次为美元 / 次）。 */
export interface Price {
  model: string;
  input: number | null;
  output: number | null;
  cache_read: number | null;
  cache_write_5m: number | null;
  cache_write_1h: number | null;
  per_request: number | null;
  source: string;
}

export interface ModelRow {
  model: string;
  /** 原厂价库里的名字（别名映射后）。 */
  official: string;
  /** 原厂价；null 表示缺价。 */
  price: Price | null;
  upstream_input: number | null;
  derived: number | null;
  deviation: number | null;
  deviates: boolean;
  reason: string;
  override: number | null;
  /** 这个模型的生效倍率；算不出为 null（此时没有成本）。 */
  effective: number | null;
  effective_source: EffectiveSource;
  cost_in_per_m: number | null;
  cost_out_per_m: number | null;
  cost_per_request: number | null;
}

/** 模型级倍率来源：模型覆盖 / 渠道手填 / 按模型推算 / 渠道倍率 / 无法推算。 */
export type EffectiveSource = 'override' | 'manual' | 'model' | 'channel' | 'none';

export interface ChannelModels {
  channel_id: number;
  name: string;
  vendor: string;
  discount: number | null;
  discount_source: DiscountSource;
  discount_reason: string;
  upstream_group_ratio: number | null;
  derived_median: number | null;
  currency_rate: number | null;
  test_model: string;
  models: ModelRow[];
}

export interface ModelDiscountInput {
  model: string;
  discount: number | null;
}

export interface CacheOverrideInput {
  read?: number | null;
  write?: number | null;
}

export type PriceStatus = 'normal' | 'deviation' | 'no_price' | 'per_request';
export type PriceStatusFilter = PriceStatus | 'all';

export interface NewapiPrice {
  input: number | null;
  output: number | null;
  cache_read: number | null;
  cache_write: number | null;
  per_request: number | null;
  unknown: string[];
}

export interface PriceDeviation {
  input: number | null;
  output: number | null;
  cache_read: number | null;
  per_request: number | null;
}

export interface PriceRow {
  model: string;
  vendor: string;
  aliases: string[];
  official: Price | null;
  source: string;
  /** null 表示 NewAPI 没有配置该模型。 */
  newapi: NewapiPrice | null;
  /** null 表示无法比较（缺价或 NewAPI 未配置）。 */
  deviation: PriceDeviation | null;
  channel_count: number;
  status: PriceStatus;
}

export interface PriceCounts {
  all: number;
  normal: number;
  deviation: number;
  no_price: number;
  per_request: number;
}

export interface PriceList {
  items: PriceRow[];
  counts: PriceCounts;
}

export interface PriceQuery {
  status?: PriceStatusFilter;
  vendor?: string;
  q?: string;
}

export interface PriceOverride {
  per_request_mode: boolean;
  input: number | null;
  output: number | null;
  cache_read: number | null;
  cache_write_5m: number | null;
  cache_write_1h: number | null;
  per_request: number | null;
  remark: string;
}

export interface PriceChannelUse {
  id: number;
  name: string;
  vendor: string;
  /** 这个模型在该渠道的生效倍率；算不出为 null。 */
  discount: number | null;
  override: number | null;
  cost_in_per_m: number | null;
  cost_out_per_m: number | null;
  cost_per_request: number | null;
}

export interface PriceDetail {
  model: string;
  official_name: string;
  official: Price | null;
  override: PriceOverride | null;
  aliases: string[];
  channels: PriceChannelUse[];
}

export interface AliasEntry {
  official: string;
  aliases: string[];
}

export type PriceSyncResult = JobStatus;

export interface DeriveModel {
  model: string;
  official: string;
  discount: number | null;
  deviation: number | null;
  deviates: boolean;
  reason: string;
}

export interface Derivation {
  median: number | null;
  models: DeriveModel[];
}

export interface DeriveChannel {
  channel_id: number;
  name: string;
  group: string;
  discount: number | null;
  discount_source: DiscountSource;
  derivation: Derivation;
}
