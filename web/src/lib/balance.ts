import { DASH, formatNumber } from './format';

/** 余额两段文字（含货币符号）及其字符数；某一段为空时对应文字为 null。 */
export interface BalanceParts {
  usd: string | null;
  cny: string | null;
}

/** 列级信息：所有行里美元、人民币文字的最大字符数（单位 ch），用来给两段定宽、对齐 ≈。 */
export interface BalanceWidths {
  usdCh: number;
  cnyCh: number;
}

/** 「 ≈ 」占的宽度（ch）。 */
export const BALANCE_SEP_CH = 3;

interface BalanceLike {
  balance_usd: number | null;
  balance_cny: number | null;
}

export function balanceParts(usd: number | null, cny: number | null): BalanceParts {
  return {
    usd: usd === null ? null : `$${formatNumber(usd)}`,
    cny: cny === null ? null : `¥${formatNumber(cny)}`,
  };
}

/** 悬停提示：完整的「$12.30 ≈ ¥88.10」；只有一边时只写那一边。 */
export function balanceTitle(parts: BalanceParts): string {
  return [parts.usd, parts.cny].filter((s): s is string => s !== null).join(' ≈ ');
}

/**
 * 按所有行算两段的最大字符数。只有人民币的行，美元位置显示「—」，也要占 1 个字符。
 * 返回新对象；调用方应只取其中的数字做依赖，保证引用稳定。
 */
export function balanceWidths(rows: ReadonlyArray<BalanceLike>): BalanceWidths {
  let usdCh = 0;
  let cnyCh = 0;
  for (const r of rows) {
    const { usd, cny } = balanceParts(r.balance_usd, r.balance_cny);
    if (usd !== null) usdCh = Math.max(usdCh, usd.length);
    else if (cny !== null) usdCh = Math.max(usdCh, DASH.length);
    if (cny !== null) cnyCh = Math.max(cnyCh, cny.length);
  }
  return { usdCh, cnyCh };
}

/** 同时放下最宽那一行「美元 ≈ 人民币」需要的宽度（ch）。没有任何人民币时不含 ≈ 和人民币。 */
export function balanceFullCh({ usdCh, cnyCh }: BalanceWidths): number {
  return cnyCh > 0 ? usdCh + BALANCE_SEP_CH + cnyCh : usdCh;
}
