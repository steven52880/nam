import type { FailClass, StatsMetrics } from '@/api/types-stats';
import { FAIL_CLASS_LABELS } from '@/api/types-stats';
import { formatSignificant } from '@/lib/format';

const DASH = '—';

export const CLASS_ORDER: FailClass[] = ['429', '5xx', 'timeout', '401', 'balance', 'other'];

export function pct(v: number | null | undefined): string {
  return v === null || v === undefined ? DASH : `${(v * 100).toFixed(1)}%`;
}

export function ms(v: number | null | undefined): string {
  return v === null || v === undefined ? DASH : String(Math.round(v));
}

export function speed(v: number | null | undefined): string {
  return v === null || v === undefined ? DASH : v.toFixed(1);
}

export function cny(v: number | null | undefined): string {
  return formatSignificant(v);
}

export function int(v: number | null | undefined): string {
  return v === null || v === undefined ? DASH : String(Math.round(v));
}

/** 吐字速度 p50，估算时加「估」。 */
export function speedWithMark(m: Pick<StatsMetrics, 'speed_p50' | 'speed_estimated'>): string {
  if (m.speed_p50 === null) return DASH;
  return m.speed_estimated ? `${speed(m.speed_p50)} 估` : speed(m.speed_p50);
}

export function failClassText(m: Pick<StatsMetrics, 'fail_by_class'>): string {
  const parts: string[] = [];
  for (const c of CLASS_ORDER) {
    const n = m.fail_by_class[c];
    if (n) parts.push(`${FAIL_CLASS_LABELS[c]}×${n}`);
  }
  return parts.length > 0 ? parts.join('，') : DASH;
}

export function formatHour(t: number): string {
  const d = new Date(t * 1000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:00`;
}
