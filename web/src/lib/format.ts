import type { JobName } from '@/api/types';

export function formatTime(unix: number): string {
  if (!unix) return '—';
  const d = new Date(unix * 1000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

/** Unix 秒 → 「x 分钟前 / x 小时前 / x 天前」；空值显示「—」。 */
export function formatAgo(unix: number, nowMs: number = Date.now()): string {
  if (!unix) return '—';
  const sec = Math.max(0, Math.floor(nowMs / 1000) - unix);
  if (sec < 60) return '刚刚';
  if (sec < 3600) return `${Math.floor(sec / 60)} 分钟前`;
  if (sec < 86400) return `${Math.floor(sec / 3600)} 小时前`;
  return `${Math.floor(sec / 86400)} 天前`;
}

export function formatDuration(ms: number): string {
  if (!ms) return '—';
  if (ms < 1000) return `${ms} 毫秒`;
  return `${(ms / 1000).toFixed(1)} 秒`;
}

export function formatNumber(v: number | null | undefined, digits = 2): string {
  if (v === null || v === undefined) return '—';
  return v.toFixed(digits);
}

/** 保留有效数字（默认 4 位），去掉多余的 0；空值显示「—」。 */
export function formatSignificant(v: number | null | undefined, digits = 4): string {
  if (v === null || v === undefined) return '—';
  if (v === 0) return '0';
  const text = Number(v.toPrecision(digits)).toString();
  return text.includes('e') ? v.toPrecision(digits) : text;
}

/** 小数形式的比例显示为带符号的百分比，如 0.123 → +12.3%。 */
export function formatPercent(fraction: number | null | undefined): string {
  if (fraction === null || fraction === undefined) return '—';
  const percent = fraction * 100;
  return `${percent > 0 ? '+' : ''}${percent.toFixed(1)}%`;
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

export const JOB_TITLES: Record<JobName, string> = {
  balance: '余额同步',
  ratio: '倍率同步',
  price_book: '价格库同步',
  announcements: '公告检查',
  pull_logs: '日志拉取',
  pricing: '定价计算',
  consistency: '一致性检查',
  cleanup: '数据清理',
  backup: '备份',
};

export function formatIntervalSeconds(s: number): string {
  if (!s) return '—';
  if (s % 86400 === 0) return `${s / 86400} 天`;
  if (s % 3600 === 0) return `${s / 3600} 小时`;
  return `${Math.round(s / 60)} 分钟`;
}
