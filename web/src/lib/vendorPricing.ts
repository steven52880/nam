import type { VendorPricingItem, VendorPricingOverview, VendorPricingState } from '@/api/types';
import { roundGroupRatio } from './format';

/** 厂商定价的纯逻辑：输入解析、换算、状态判断。界面和测试共用。 */

/** 售价上限（原厂价的倍数）：10 = 1000%。 */
export const SELL_PRICE_MAX = 10;
/** 加价比例上限。 */
export const MARKUP_MAX = 100;

export type ParseResult = { ok: true; value: number } | { ok: false; error: string };

const DECIMAL_RE = /^(\d+(\.\d*)?|\.\d+)$/;

/** 文字 → 数字；去掉空格和结尾的「%」。不是十进制正数写法返回 null。 */
function parseDecimal(text: string, percent: boolean): number | null {
  let t = text.trim();
  if (percent && t.endsWith('%')) t = t.slice(0, -1).trim();
  if (!DECIMAL_RE.test(t)) return null;
  const v = Number(t);
  return Number.isFinite(v) ? v : null;
}

/** 百分比文字（"30" 或 "30%"）→ 小数比例（0.3），保留到 0.01%（4 位小数）。 */
export function percentTextToRatio(text: string): number | null {
  const v = parseDecimal(text, true);
  return v === null ? null : Math.round(v * 100) / 10000;
}

/** 小数比例 → 输入框里的百分比文字：0.3 → "30"，0.0215 → "2.15"。 */
export function ratioToPercentText(ratio: number): string {
  return String(Math.round(ratio * 10000) / 100);
}

/** 售价输入（百分比文字）：必须 > 0 且 ≤ 1000%。 */
export function parseSellPriceInput(text: string): ParseResult {
  const v = percentTextToRatio(text);
  if (v === null) return { ok: false, error: '请输入百分比数字，如 30' };
  if (v <= 0) return { ok: false, error: '售价必须大于 0' };
  if (v > SELL_PRICE_MAX) return { ok: false, error: `售价不能超过 ${SELL_PRICE_MAX * 100}%` };
  return { ok: true, value: v };
}

/** 加价比例输入（倍数）：必须 > 0 且 ≤ 100，保留 4 位小数。 */
export function parseMarkupInput(text: string): ParseResult {
  const v = parseDecimal(text, false);
  if (v === null) return { ok: false, error: '请输入数字，如 1.5' };
  const rounded = Math.round(v * 10000) / 10000;
  if (rounded <= 0) return { ok: false, error: '加价比例必须大于 0' };
  if (rounded > MARKUP_MAX) return { ok: false, error: `加价比例不能超过 ${MARKUP_MAX}` };
  return { ok: true, value: rounded };
}

/** 设置页的百分比输入：≥ min，且 < maxExclusive（都按百分数写）。返回小数比例。 */
export function parsePercentSetting(text: string, minPercent: number, maxExclusivePercent: number): ParseResult {
  const v = parseDecimal(text, false);
  if (v === null) return { ok: false, error: '请输入百分比数字' };
  if (v < minPercent || v >= maxExclusivePercent) {
    return { ok: false, error: `需在 ${minPercent}% 到 ${maxExclusivePercent}% 之间（不含 ${maxExclusivePercent}%）` };
  }
  return { ok: true, value: Math.round(v * 100) / 10000 };
}

/** 变化保护阈值：大于 0%（后端要求 > 0）。 */
export function parseThreshold(text: string): ParseResult {
  const r = parsePercentSetting(text, 0, 100000);
  if (r.ok && r.value <= 0) return { ok: false, error: '需要大于 0%' };
  return r;
}

/** 最小变动：0% 到 100%（不含 100%）。 */
export function parseMinChange(text: string): ParseResult {
  return parsePercentSetting(text, 0, 100);
}

/** 折合人民币：售价（原厂价的倍数）× 美元参考汇率 = 每原厂 $1 要收多少元。 */
export function cnyPerUsd(price: number, usdRate: number): number {
  return price * usdRate;
}

/** 厂商分组倍率 = 售价 × 美元参考汇率 ÷ 站点汇率，四舍五入到 3 位小数（和后端一致）。 */
export function targetGroupRatio(price: number, usdRate: number, siteRate: number): number {
  return roundGroupRatio((price * usdRate) / siteRate);
}

/** 倍率输入（小数，如 0.309）：先四舍五入到 3 位小数，必须 > 0。 */
export function parseGroupRatioInput(text: string): ParseResult {
  const v = parseDecimal(text, false);
  if (v === null) return { ok: false, error: '请输入小数，如 0.309' };
  const rounded = roundGroupRatio(v);
  if (rounded <= 0) return { ok: false, error: '倍率必须大于 0' };
  return { ok: true, value: rounded };
}

/** 倍率 → 输入框里的文字：0.30857 → "0.309"。 */
export function groupRatioToText(ratio: number): string {
  return String(roundGroupRatio(ratio));
}

/** 悬停提示：倍率的原值（显示只保留 3 位小数）。 */
export function rawRatioTitle(v: number | null): string | undefined {
  return v === null ? undefined : `原值 ${v}`;
}

/** 采纳 NewAPI 值：售价 = NewAPI 倍率 × 站点汇率 ÷ 美元参考汇率，保留 4 位小数。 */
export function adoptedPrice(newapiRatio: number, siteRate: number, usdRate: number): number {
  return Math.round(((newapiRatio * siteRate) / usdRate) * 10000) / 10000;
}

/** 当前售价：固定看手填值，跟随看计算出来的目标售价。 */
export function currentPrice(item: VendorPricingItem): number | null {
  return item.mode === 'fixed' ? item.sell_price : item.target_price;
}

/** 变化的方向：目标售价相对上次写入售价；没法算时退回后端给的绝对值。 */
export function signedChange(item: Pick<VendorPricingItem, 'target_price' | 'last_written_price' | 'change'>): number | null {
  const { target_price: target, last_written_price: last } = item;
  if (target !== null && last !== null && last > 0) return (target - last) / last;
  return item.change;
}

/** 需要主人处理的厂商：待确认或漂移。 */
export function needsAttention(state: VendorPricingState): boolean {
  return state === 'pending' || state === 'drift';
}

/** 总览告警和导航徽标用：待确认 + 漂移的厂商数。 */
export function attentionCount(items: readonly Pick<VendorPricingItem, 'state'>[]): number {
  return items.filter((i) => needsAttention(i.state)).length;
}

export interface AttentionSummary {
  pending: number;
  drift: number;
  total: number;
}

export function attentionSummary(items: readonly Pick<VendorPricingItem, 'state'>[]): AttentionSummary {
  const pending = items.filter((i) => i.state === 'pending').length;
  const drift = items.filter((i) => i.state === 'drift').length;
  return { pending, drift, total: pending + drift };
}

/** 能不能点「预览写入」：NewAPI 选项能解析，并且有待写入的厂商或分组有变化。 */
export function canPreviewWrite(o: Pick<VendorPricingOverview, 'error' | 'items' | 'groups'> | undefined): boolean {
  if (!o || o.error) return false;
  return o.items.some((i) => i.state === 'to_write') || o.groups.changed;
}

/** 选项 JSON 美化：能解析就缩进显示，不能就原样。 */
export function prettyJson(text: string): string {
  if (text.trim() === '') return '（空）';
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}
