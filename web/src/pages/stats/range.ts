import type { StatsBucket } from '@/api/types-stats';

export type Preset = '24h' | '7d' | '30d' | 'custom';

export interface TimeRange {
  from: number;
  to: number;
  bucket: StatsBucket;
}

export type RangeResult = { ok: true; range: TimeRange } | { ok: false; error: string };

export const PRESET_LABELS: Array<[Preset, string]> = [
  ['24h', '24 小时'],
  ['7d', '7 天'],
  ['30d', '30 天'],
  ['custom', '自定义'],
];

const MAX_SPAN_SEC = 186 * 86400;

export function toDateInput(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

/** 'yyyy-mm-dd' → 本地当天 0 点。 */
function parseLocalDay(s: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  if (!m) return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

function localMidnight(nowMs: number, daysAgo: number): number {
  const d = new Date(nowMs);
  return Math.floor(new Date(d.getFullYear(), d.getMonth(), d.getDate() - daysAgo).getTime() / 1000);
}

export function buildRange(preset: Preset, start: string, end: string, nowMs: number): RangeResult {
  const nowSec = Math.floor(nowMs / 1000);
  if (preset === '24h') return { ok: true, range: { from: nowSec - 86400, to: nowSec, bucket: 'hour' } };
  if (preset === '7d') return { ok: true, range: { from: localMidnight(nowMs, 6), to: nowSec, bucket: 'day' } };
  if (preset === '30d') return { ok: true, range: { from: localMidnight(nowMs, 29), to: nowSec, bucket: 'day' } };

  const s = parseLocalDay(start);
  const e = parseLocalDay(end);
  if (!s || !e) return { ok: false, error: '请选择起止日期' };
  const from = Math.floor(s.getTime() / 1000);
  const to = Math.floor(new Date(e.getFullYear(), e.getMonth(), e.getDate() + 1).getTime() / 1000);
  if (to <= from) return { ok: false, error: '结束日期不能早于开始日期' };
  if (to - from > MAX_SPAN_SEC) return { ok: false, error: '时间跨度不能超过 186 天' };
  return { ok: true, range: { from, to, bucket: 'day' } };
}
