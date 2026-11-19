import { describe, expect, it } from 'vitest';
import type { FailClass } from '@/api/types';
import {
  DASH,
  formatAgo,
  formatDate,
  formatDiscount,
  formatDuration,
  formatFailClasses,
  formatHour,
  formatInt,
  formatIntervalSeconds,
  formatMs,
  formatNumber,
  formatPercent,
  formatSignificant,
  formatSpeed,
  formatTime,
  showValue,
} from './format';
import { FAIL_CLASS, JOB_TITLES, labelOf, optionsOf } from './labels';

// ─────────────────────────────────────────────────────────────
// legacy：迁移前各函数的原实现，原样复制（括号内是来源文件:行号，行号为迁移前）。
// 新函数的默认输出必须与这里逐字一致。
// ─────────────────────────────────────────────────────────────

/** legacy: pages/stats/fmt.ts:127 → api/types-stats.ts:127-134 FAIL_CLASS_LABELS */
const LEGACY_FAIL_CLASS_LABELS: Record<FailClass, string> = {
  '429': '429 限流',
  '5xx': '5xx',
  timeout: '超时',
  '401': '401 / 403',
  balance: '余额不足',
  other: '其他',
};

// ── pages/stats/fmt.ts ──
const LEGACY_DASH = '—';
const CLASS_ORDER: FailClass[] = ['429', '5xx', 'timeout', '401', 'balance', 'other']; // fmt.ts:7

/** legacy: pages/stats/fmt.ts:9-11 */
function legacyPct(v: number | null | undefined): string {
  return v === null || v === undefined ? LEGACY_DASH : `${(v * 100).toFixed(1)}%`;
}
/** legacy: pages/stats/fmt.ts:13-15 */
function legacyMsPlain(v: number | null | undefined): string {
  return v === null || v === undefined ? LEGACY_DASH : String(Math.round(v));
}
/** legacy: pages/stats/fmt.ts:17-19 */
function legacySpeedPlain(v: number | null | undefined): string {
  return v === null || v === undefined ? LEGACY_DASH : v.toFixed(1);
}
/** legacy: pages/stats/fmt.ts:25-27 */
function legacyInt(v: number | null | undefined): string {
  return v === null || v === undefined ? LEGACY_DASH : String(Math.round(v));
}
/** legacy: pages/stats/fmt.ts:30-33 */
function legacySpeedWithMark(m: { speed_p50: number | null; speed_estimated: boolean }): string {
  if (m.speed_p50 === null) return LEGACY_DASH;
  return m.speed_estimated ? `${legacySpeedPlain(m.speed_p50)} 估` : legacySpeedPlain(m.speed_p50);
}
/** legacy: pages/stats/fmt.ts:35-42 */
function legacyFailClassCompact(m: { fail_by_class: Partial<Record<FailClass, number>> }): string {
  const parts: string[] = [];
  for (const c of CLASS_ORDER) {
    const n = m.fail_by_class[c];
    if (n) parts.push(`${LEGACY_FAIL_CLASS_LABELS[c]}×${n}`);
  }
  return parts.length > 0 ? parts.join('，') : LEGACY_DASH;
}
/** legacy: pages/stats/fmt.ts:44-48 */
function legacyFormatHour(t: number): string {
  const d = new Date(t * 1000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:00`;
}

// ── pages/channels/StatCells.tsx ──
/** legacy: pages/channels/StatCells.tsx:9-12 */
function legacyFormatRate(fraction: number | null | undefined): string {
  if (fraction === null || fraction === undefined) return '—';
  return `${(fraction * 100).toFixed(1)}%`;
}
/** legacy: pages/channels/StatCells.tsx:14-17 */
function legacyFormatMs(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—';
  return `${Math.round(value)} ms`;
}
/** legacy: pages/channels/StatCells.tsx:19-22 */
function legacyFormatSpeed(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—';
  return `${value.toFixed(1)} t/s`;
}
/** legacy: pages/channels/StatCells.tsx:24-29（FAIL_CLASS_LABELS 取 types-stats 原表） */
function legacyFailClassList(metrics: { fail_by_class: Partial<Record<FailClass, number>> }): string {
  const parts = (Object.keys(LEGACY_FAIL_CLASS_LABELS) as FailClass[])
    .filter((key) => (metrics.fail_by_class[key] ?? 0) > 0)
    .map((key) => `${LEGACY_FAIL_CLASS_LABELS[key]} ${metrics.fail_by_class[key]}`);
  return parts.length > 0 ? parts.join('、') : '无';
}

// ── pages/channels/edits.ts ──
/** legacy: pages/channels/edits.ts:87-89 */
function legacyFormatDiscount(value: number): string {
  return String(Number(value.toFixed(4)));
}

// ── pages/stats/Leaderboard.tsx ──
/** legacy: pages/stats/Leaderboard.tsx:44-47 */
function legacyYmd(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}`;
}
/** legacy: pages/stats/Leaderboard.tsx:30（speed 列的 show，speed 即 fmt.ts 的 speed） */
function legacyLeaderboardSpeed(r: { speed_p50: number | null; speed_estimated: boolean }): string {
  return r.speed_p50 === null ? '—' : `${legacySpeedPlain(r.speed_p50)}${r.speed_estimated ? ' 估' : ''}`;
}

// ── components/entities/ApplyDialog.tsx ──
/** legacy: 原 pages/channels/DriftDialog.tsx（已删除，函数现用于 ApplyDialog） */
function legacyShowValue(v: unknown): string {
  if (v === null || v === undefined || v === '') return '（空）';
  if (Array.isArray(v)) return v.join(', ') || '（空）';
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}

// ── lib/format.ts（迁移前）──
/** legacy: lib/format.ts:26-29 */
function legacyFormatNumber(v: number | null | undefined, digits = 2): string {
  if (v === null || v === undefined) return '—';
  return v.toFixed(digits);
}
/** legacy: lib/format.ts:40-44（带正号的百分比，新函数需 signed:true） */
function legacyFormatPercentSigned(fraction: number | null | undefined): string {
  if (fraction === null || fraction === undefined) return '—';
  const percent = fraction * 100;
  return `${percent > 0 ? '+' : ''}${percent.toFixed(1)}%`;
}
/** legacy: lib/format.ts:32-37 */
function legacyFormatSignificant(v: number | null | undefined, digits = 4): string {
  if (v === null || v === undefined) return '—';
  if (v === 0) return '0';
  const text = Number(v.toPrecision(digits)).toString();
  return text.includes('e') ? v.toPrecision(digits) : text;
}
/** legacy: lib/format.ts:3-8 */
function legacyFormatTime(unix: number): string {
  if (!unix) return '—';
  const d = new Date(unix * 1000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}
/** legacy: lib/format.ts:11-18 */
function legacyFormatAgo(unix: number, nowMs: number): string {
  if (!unix) return '—';
  const sec = Math.max(0, Math.floor(nowMs / 1000) - unix);
  if (sec < 60) return '刚刚';
  if (sec < 3600) return `${Math.floor(sec / 60)} 分钟前`;
  if (sec < 86400) return `${Math.floor(sec / 3600)} 小时前`;
  return `${Math.floor(sec / 86400)} 天前`;
}
/** legacy: lib/format.ts:20-24 */
function legacyFormatDuration(ms: number): string {
  if (!ms) return '—';
  if (ms < 1000) return `${ms} 毫秒`;
  return `${(ms / 1000).toFixed(1)} 秒`;
}
/** legacy: lib/format.ts:88-93 */
function legacyFormatIntervalSeconds(s: number): string {
  if (!s) return '—';
  if (s % 86400 === 0) return `${s / 86400} 天`;
  if (s % 3600 === 0) return `${s / 3600} 小时`;
  return `${Math.round(s / 60)} 分钟`;
}
/** legacy: lib/format.ts:76-86 */
const LEGACY_JOB_TITLES = {
  balance: '余额同步',
  ratio: '倍率同步',
  price_book: '价格库同步',
  announcements: '公告检查',
  pull_logs: '日志拉取',
  pricing: '定价计算',
  consistency: '规则检查',
  cleanup: '数据清理',
  backup: '备份',
};

// ── 内联写法 ──
/** legacy: pages/channels/columns.tsx:94、CacheTestDialog.tsx:69（rate 非空） */
const legacyInlineRate = (rate: number): string => `${(rate * 100).toFixed(1)}%`;
/** legacy: pages/channels/CacheTestDialog.tsx:75,121（rate 可空） */
const legacyInlineRateNullable = (rate: number | null): string => (rate === null ? '—' : `${(rate * 100).toFixed(1)}%`);

// ─────────────────────────────────────────────────────────────
// 典型输入
// ─────────────────────────────────────────────────────────────
const NUMS: Array<number | null | undefined> = [
  null,
  undefined,
  0,
  -0,
  1,
  -1,
  0.5,
  -0.5,
  0.123,
  -0.123,
  0.0004,
  0.0005,
  0.05,
  0.9999,
  1.005,
  2.675,
  12.345,
  42,
  42.55,
  850.4,
  850.5,
  1000,
  123456.789,
  1e7,
  1e21,
  0.000012345,
  -123456.789,
];

describe('DASH', () => {
  it('是长破折号', () => {
    expect(DASH).toBe('—');
  });
});

describe('formatPercent 默认输出 === legacy', () => {
  it.each(NUMS)('%s', (v) => {
    expect(formatPercent(v)).toBe(legacyFormatRate(v));
    expect(formatPercent(v)).toBe(legacyPct(v));
  });
  it('内联写法（非空）', () => {
    for (const v of NUMS) if (typeof v === 'number') expect(formatPercent(v)).toBe(legacyInlineRate(v));
  });
  it('内联写法（可空）', () => {
    for (const v of NUMS) if (v !== undefined) expect(formatPercent(v)).toBe(legacyInlineRateNullable(v));
  });
  it('signed:true === 旧 formatPercent（带正号）', () => {
    for (const v of NUMS) expect(formatPercent(v, { signed: true })).toBe(legacyFormatPercentSigned(v));
  });
  it('可选参数', () => {
    expect(formatPercent(0.123, { signed: true })).toBe('+12.3%');
    expect(formatPercent(-0.123, { signed: true })).toBe('-12.3%');
    expect(formatPercent(0, { signed: true })).toBe('0.0%');
    expect(formatPercent(0.123)).toBe('12.3%');
    expect(formatPercent(0.12345, { digits: 2 })).toBe('12.35%');
    expect(formatPercent(0.5, { digits: 0 })).toBe('50%');
    expect(formatPercent(0.5, { digits: 0, signed: true })).toBe('+50%');
    expect(formatPercent(null, { signed: true })).toBe(DASH);
  });
});

describe('formatMs 默认输出 === legacy', () => {
  it.each(NUMS)('%s', (v) => {
    expect(formatMs(v)).toBe(legacyFormatMs(v));
  });
  it('unit:false === fmt.ms', () => {
    for (const v of NUMS) expect(formatMs(v, { unit: false })).toBe(legacyMsPlain(v));
  });
  it('可选参数', () => {
    expect(formatMs(850.4)).toBe('850 ms');
    expect(formatMs(850.4, { unit: false })).toBe('850');
    expect(formatMs(850.4, { unit: true })).toBe('850 ms');
    expect(formatMs(null, { unit: false })).toBe(DASH);
  });
});

describe('formatSpeed 默认输出 === legacy', () => {
  it.each(NUMS)('%s', (v) => {
    expect(formatSpeed(v)).toBe(legacyFormatSpeed(v));
  });
  it('unit:false === fmt.speed', () => {
    for (const v of NUMS) expect(formatSpeed(v, { unit: false })).toBe(legacySpeedPlain(v));
  });
  it('estimated + unit:false === fmt.speedWithMark / Leaderboard 的 speed 列', () => {
    for (const v of NUMS) {
      for (const estimated of [true, false]) {
        const metrics = { speed_p50: v ?? null, speed_estimated: estimated };
        expect(formatSpeed(metrics.speed_p50, { unit: false, estimated })).toBe(legacySpeedWithMark(metrics));
        expect(formatSpeed(metrics.speed_p50, { unit: false, estimated })).toBe(legacyLeaderboardSpeed(metrics));
      }
    }
  });
  it('可选参数', () => {
    expect(formatSpeed(42)).toBe('42.0 t/s');
    expect(formatSpeed(42, { unit: false })).toBe('42.0');
    expect(formatSpeed(42, { estimated: true })).toBe('42.0 t/s 估');
    expect(formatSpeed(null, { estimated: true })).toBe(DASH);
  });
});

describe('formatInt === legacy fmt.int', () => {
  it.each(NUMS)('%s', (v) => {
    expect(formatInt(v)).toBe(legacyInt(v));
  });
});

describe('formatHour === legacy fmt.formatHour', () => {
  it.each([0, 1, 3599, 3600, 1700000000, 1735689600, 1767225599, 86400 * 365])('%s', (t) => {
    expect(formatHour(t)).toBe(legacyFormatHour(t));
  });
});

describe('formatDate === legacy Leaderboard.ymd', () => {
  it.each([
    new Date(2026, 0, 1),
    new Date(2026, 10, 17, 23, 59, 59),
    new Date(1999, 11, 31),
    new Date(2024, 1, 29, 12),
    new Date(0),
  ])('%s', (d) => {
    expect(formatDate(d)).toBe(legacyYmd(d));
  });
});

describe('formatDiscount === legacy edits.formatDiscount', () => {
  it.each([0, 1, -1, 0.5, 0.85, 0.123456, 0.99995, 1.00005, 2.5, 10, 123456.789, 1e-5, 0.1 + 0.2])('%s', (v) => {
    expect(formatDiscount(v)).toBe(legacyFormatDiscount(v));
  });
});

describe('formatFailClasses', () => {
  const samples: Array<Partial<Record<FailClass, number>>> = [
    {},
    { '429': 3 },
    { '5xx': 1, timeout: 2 },
    { '401': 5, balance: 1, other: 9 },
    { '429': 1, '5xx': 2, timeout: 3, '401': 4, balance: 5, other: 6 },
    { '429': 0, other: 0 },
    { '429': -1, '5xx': 2 },
  ];
  it.each(samples)('默认（list）=== StatCells.failClassText：%j', (fail_by_class) => {
    expect(formatFailClasses({ fail_by_class })).toBe(legacyFailClassList({ fail_by_class }));
    expect(formatFailClasses({ fail_by_class }, { style: 'list' })).toBe(legacyFailClassList({ fail_by_class }));
  });
  it.each(samples)('compact === fmt.failClassText：%j', (fail_by_class) => {
    expect(formatFailClasses({ fail_by_class }, { style: 'compact' })).toBe(legacyFailClassCompact({ fail_by_class }));
  });
  it('文字', () => {
    expect(formatFailClasses({ fail_by_class: {} })).toBe('无');
    expect(formatFailClasses({ fail_by_class: {} }, { style: 'compact' })).toBe(DASH);
    expect(formatFailClasses({ fail_by_class: { '429': 3, timeout: 2 } })).toBe('429 限流 3、超时 2');
    expect(formatFailClasses({ fail_by_class: { '429': 3, timeout: 2 } }, { style: 'compact' })).toBe('429 限流×3，超时×2');
  });
});

describe('FAIL_CLASS 文字与旧 FAIL_CLASS_LABELS 一致', () => {
  it('逐项', () => {
    for (const k of Object.keys(LEGACY_FAIL_CLASS_LABELS) as FailClass[]) {
      expect(FAIL_CLASS[k].label).toBe(LEGACY_FAIL_CLASS_LABELS[k]);
    }
    expect(Object.keys(FAIL_CLASS)).toEqual(Object.keys(LEGACY_FAIL_CLASS_LABELS));
  });
});

describe('showValue === legacy showValue', () => {
  const values: unknown[] = [
    null,
    undefined,
    '',
    'abc',
    0,
    1.5,
    -3,
    true,
    false,
    [],
    ['a', 'b'],
    [1, 2, 3],
    [''],
    {},
    { a: 1, b: [1, 2] },
    NaN,
  ];
  it.each(values.map((v) => [v]))('%j', (v) => {
    expect(showValue(v)).toBe(legacyShowValue(v));
  });
});

describe('formatNumber 默认输出 === legacy', () => {
  it.each(NUMS)('%s（默认位数）', (v) => {
    expect(formatNumber(v)).toBe(legacyFormatNumber(v));
  });
  it.each(NUMS)('%s（3 位）', (v) => {
    expect(formatNumber(v, 3)).toBe(legacyFormatNumber(v, 3));
    expect(formatNumber(v, 0)).toBe(legacyFormatNumber(v, 0));
    expect(formatNumber(v, 4)).toBe(legacyFormatNumber(v, 4));
  });
  it('可选参数 trim / group', () => {
    expect(formatNumber(1.5, 3, { trim: true })).toBe('1.5');
    expect(formatNumber(10, 2, { trim: true })).toBe('10');
    expect(formatNumber(0, 2, { trim: true })).toBe('0');
    expect(formatNumber(0.05, 3, { trim: true })).toBe('0.05');
    expect(formatNumber(100, 0, { trim: true })).toBe('100');
    expect(formatNumber(1234567.891, 2, { group: true })).toBe('1,234,567.89');
    expect(formatNumber(-1234567, 0, { group: true })).toBe('-1,234,567');
    expect(formatNumber(999, 2, { group: true })).toBe('999.00');
    expect(formatNumber(1234.5, 3, { trim: true, group: true })).toBe('1,234.5');
    expect(formatNumber(null, 2, { trim: true, group: true })).toBe(DASH);
  });
});

describe('formatSignificant / formatTime / formatAgo / formatDuration / formatIntervalSeconds 未变', () => {
  it.each(NUMS)('formatSignificant %s', (v) => {
    expect(formatSignificant(v)).toBe(legacyFormatSignificant(v));
    expect(formatSignificant(v, 3)).toBe(legacyFormatSignificant(v, 3));
  });
  it.each([0, 1, 1700000000, 1767225599])('formatTime %s', (t) => {
    expect(formatTime(t)).toBe(legacyFormatTime(t));
  });
  it.each([0, 1700000000, 1700000030, 1700000600, 1700007200, 1700200000, 1800000000])('formatAgo %s', (t) => {
    const now = 1700000100 * 1000;
    expect(formatAgo(t, now)).toBe(legacyFormatAgo(t, now));
  });
  it.each([0, 1, 999, 1000, 1500, 123456])('formatDuration %s', (ms) => {
    expect(formatDuration(ms)).toBe(legacyFormatDuration(ms));
  });
  it.each([0, 30, 60, 90, 3600, 7200, 86400, 172800, 5400])('formatIntervalSeconds %s', (s) => {
    expect(formatIntervalSeconds(s)).toBe(legacyFormatIntervalSeconds(s));
  });
});

describe('JOB_TITLES 与迁移前一致', () => {
  it('逐项', () => {
    expect(JOB_TITLES).toEqual(LEGACY_JOB_TITLES);
  });
});

describe('labels', () => {
  it('labelOf 找不到时用键做文字', () => {
    expect(labelOf({ 1: { label: '启用', tone: 'success' } }, 1)).toEqual({ label: '启用', tone: 'success' });
    expect(labelOf({ 1: { label: '启用', tone: 'success' } }, 9)).toEqual({ label: '9', tone: 'neutral' });
    expect(labelOf({}, 'x').label).toBe('x');
  });
  it('optionsOf 默认按字典顺序，keys 可指定顺序和子集', () => {
    const dict = { a: { label: 'A', tone: 'neutral' as const }, b: { label: 'B', tone: 'neutral' as const } };
    expect(optionsOf(dict)).toEqual([['a', 'A'], ['b', 'B']]);
    expect(optionsOf(dict, ['b'])).toEqual([['b', 'B']]);
    expect(optionsOf(dict, ['b', 'a'])).toEqual([['b', 'B'], ['a', 'A']]);
  });
});
