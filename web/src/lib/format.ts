import type { FailClass } from '@/api/types';
import { FAIL_CLASS, FAIL_CLASS_ORDER } from './labels';

/** 空值占位符：所有格式化函数遇到 null / undefined 都返回它。 */
export const DASH = '—';

export function formatTime(unix: number): string {
  if (!unix) return DASH;
  const d = new Date(unix * 1000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

/** Unix 秒 → 「x 分钟前 / x 小时前 / x 天前」；空值显示「—」。 */
export function formatAgo(unix: number, nowMs: number = Date.now()): string {
  if (!unix) return DASH;
  const sec = Math.max(0, Math.floor(nowMs / 1000) - unix);
  if (sec < 60) return '刚刚';
  if (sec < 3600) return `${Math.floor(sec / 60)} 分钟前`;
  if (sec < 86400) return `${Math.floor(sec / 3600)} 小时前`;
  return `${Math.floor(sec / 86400)} 天前`;
}

export function formatDuration(ms: number): string {
  if (!ms) return DASH;
  if (ms < 1000) return `${ms} 毫秒`;
  return `${(ms / 1000).toFixed(1)} 秒`;
}

/** 默认 toFixed(digits)；trim：去掉小数部分末尾的 0；group：整数部分加千分位逗号。 */
export function formatNumber(
  v: number | null | undefined,
  digits = 2,
  opts?: { trim?: boolean; group?: boolean },
): string {
  if (v === null || v === undefined) return DASH;
  let text = v.toFixed(digits);
  if (opts?.trim && text.includes('.')) text = text.replace(/\.?0+$/, '');
  if (opts?.group) {
    const [int, frac] = text.split('.');
    const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    text = frac === undefined ? grouped : `${grouped}.${frac}`;
  }
  return text;
}

/** 保留有效数字（默认 4 位），去掉多余的 0；空值显示「—」。 */
export function formatSignificant(v: number | null | undefined, digits = 4): string {
  if (v === null || v === undefined) return DASH;
  if (v === 0) return '0';
  const text = Number(v.toPrecision(digits)).toString();
  return text.includes('e') ? v.toPrecision(digits) : text;
}

/** 0–1 的比率 → 百分比，默认 1 位小数、不带正号；signed 时正数带「+」（如 0.123 → +12.3%）。 */
export function formatPercent(
  ratio: number | null | undefined,
  opts?: { digits?: number; signed?: boolean },
): string {
  if (ratio === null || ratio === undefined) return DASH;
  const percent = ratio * 100;
  const sign = opts?.signed && percent > 0 ? '+' : '';
  return `${sign}${percent.toFixed(opts?.digits ?? 1)}%`;
}

/**
 * 「原厂价的倍数」→ 百分比：0.024 → 2.4%，0.0085 → 0.85%，1.5 → 150%。
 * 按有效数字（默认 3 位）取，去掉多余的 0；空值显示「—」。
 */
export function formatRatioPercent(v: number | null | undefined, digits = 3): string {
  if (v === null || v === undefined) return DASH;
  return `${formatSignificant(v * 100, digits)}%`;
}

/** 厂商分组倍率统一保留的小数位数。 */
export const GROUP_RATIO_DIGITS = 3;

/** 厂商分组倍率四舍五入到 3 位小数：0.30857 → 0.309。 */
export function roundGroupRatio(v: number): number {
  const f = 10 ** GROUP_RATIO_DIGITS;
  return Math.round(v * f) / f;
}

/** 厂商分组倍率显示：最多 3 位小数（四舍五入），去掉多余的 0；空值显示「—」。 */
export function formatGroupRatio(v: number | null | undefined): string {
  if (v === null || v === undefined) return DASH;
  return formatNumber(roundGroupRatio(v), GROUP_RATIO_DIGITS, { trim: true });
}

/** 毫秒：默认「850 ms」；unit:false 只返回数字。 */
export function formatMs(v: number | null | undefined, opts?: { unit?: boolean }): string {
  if (v === null || v === undefined) return DASH;
  return opts?.unit === false ? String(Math.round(v)) : `${Math.round(v)} ms`;
}

/** 吐字速度：默认「42.0 t/s」；unit:false 只返回数字；estimated 时末尾加「 估」。 */
export function formatSpeed(v: number | null | undefined, opts?: { unit?: boolean; estimated?: boolean }): string {
  if (v === null || v === undefined) return DASH;
  const text = opts?.unit === false ? v.toFixed(1) : `${v.toFixed(1)} t/s`;
  return opts?.estimated ? `${text} 估` : text;
}

export function formatInt(v: number | null | undefined): string {
  if (v === null || v === undefined) return DASH;
  return String(Math.round(v));
}

/** Unix 秒 → 「MM-DD HH:00」。 */
export function formatHour(sec: number): string {
  const d = new Date(sec * 1000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:00`;
}

/** Date → 「YYYYMMDD」（用于导出文件名）。 */
export function formatDate(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}`;
}

/** 倍率（上游分组倍率）：最多 4 位小数，去掉多余的 0。 */
export function formatDiscount(v: number): string {
  return String(Number(v.toFixed(4)));
}

/**
 * 失败类别。list：「名称 数量」用「、」连接，空为「无」；
 * compact：「名称×数量」用「，」连接，空为 DASH。
 */
export function formatFailClasses(
  m: { fail_by_class: Partial<Record<FailClass, number>> },
  opts?: { style?: 'list' | 'compact' },
): string {
  if (opts?.style === 'compact') {
    const parts: string[] = [];
    for (const c of FAIL_CLASS_ORDER) {
      const n = m.fail_by_class[c];
      if (n) parts.push(`${FAIL_CLASS[c].label}×${n}`);
    }
    return parts.length > 0 ? parts.join('，') : DASH;
  }
  const parts = (Object.keys(FAIL_CLASS) as FailClass[])
    .filter((key) => (m.fail_by_class[key] ?? 0) > 0)
    .map((key) => `${FAIL_CLASS[key].label} ${m.fail_by_class[key]}`);
  return parts.length > 0 ? parts.join('、') : '无';
}

/** 显示一个值（应用变更的前后对比用）：空值显示「（空）」，数组用逗号连接，对象转 JSON。 */
export function showValue(v: unknown): string {
  if (v === null || v === undefined || v === '') return '（空）';
  if (Array.isArray(v)) return v.join(', ') || '（空）';
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}

export type DurationUnit = 'm' | 'h' | 'd';

export interface ParsedInterval {
  value: number;
  unit: DurationUnit;
}

/** 解析 Go duration（如 30m0s、6h0m0s、24h）为数值 + 单位。 */
export function parseGoDuration(s: string | undefined): ParsedInterval | null {
  if (!s) return null;
  const re = /(\d+(?:\.\d+)?)(h|m|s)/g;
  let seconds = 0;
  let matched = false;
  for (const m of s.matchAll(re)) {
    matched = true;
    const n = Number(m[1]);
    seconds += m[2] === 'h' ? n * 3600 : m[2] === 'm' ? n * 60 : n;
  }
  if (!matched || seconds <= 0) return null;
  if (seconds % 86400 === 0) return { value: seconds / 86400, unit: 'd' };
  if (seconds % 3600 === 0) return { value: seconds / 3600, unit: 'h' };
  return { value: Math.max(1, Math.round(seconds / 60)), unit: 'm' };
}

/** 序列化为 Go duration："30m" / "6h" / "24h"（天换算为小时）。 */
export function toGoDuration(v: ParsedInterval): string {
  if (v.unit === 'd') return `${v.value * 24}h`;
  return `${v.value}${v.unit}`;
}

export function formatIntervalSeconds(s: number): string {
  if (!s) return DASH;
  if (s % 86400 === 0) return `${s / 86400} 天`;
  if (s % 3600 === 0) return `${s / 3600} 小时`;
  return `${Math.round(s / 60)} 分钟`;
}
