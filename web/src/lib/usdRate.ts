/** 美元参考汇率（设置项 usd_cny_rate，元/$）：用于把成本换算成原厂价的百分比。 */
export const USD_CNY_RATE_KEY = 'usd_cny_rate';
export const DEFAULT_USD_CNY_RATE = '7.2';
export const MAX_USD_CNY_RATE = 100;

/** 输入框文字是否是合法的美元参考汇率：十进制正数，且 ≤ 100。 */
export function isUsdCnyRateValid(text: string): boolean {
  const t = text.trim();
  if (!/^\d+(\.\d+)?$|^\.\d+$/.test(t)) return false;
  const v = Number(t);
  return v > 0 && v <= MAX_USD_CNY_RATE;
}

/** 设置里的参考汇率文字 → 数字；没取到或不合法时用默认 7.2。 */
export function parseUsdCnyRate(text: string | null | undefined): number {
  return text != null && isUsdCnyRateValid(text) ? Number(text.trim()) : Number(DEFAULT_USD_CNY_RATE);
}

/** 上游渠道售价 = 分组倍率 × 上游货币汇率 ÷ 美元参考汇率（原厂价的倍数，显示成百分比）。 */
export function channelPrice(groupRatio: number, currencyRate: number, usdCnyRate: number): number {
  return (groupRatio * currencyRate) / usdCnyRate;
}

/** 「上游渠道售价」列的表头悬停说明，渠道页和令牌 / 分组选择处共用。 */
export const CHANNEL_PRICE_HINT =
  '= 分组倍率 × 上游货币汇率 ÷ 美元参考汇率，表示实际花费是原厂价的百分之几。例：0.35 × 0.5 ÷ 7.2 ≈ 2.4%';
