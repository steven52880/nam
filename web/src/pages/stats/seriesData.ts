import type { SeriesPoint, StatsBucket } from '@/api/types-stats';
import { formatHour } from '@/lib/format';

export interface SeriesDatum {
  t: number;
  label: string;
  requests: number;
  /** 成功率，0–100。 */
  rate: number | null;
  frt50: number | null;
  frt95: number | null;
  sp50: number | null;
  sp95: number | null;
  /** 该点吐字速度是否为按整秒估算。 */
  speedEst: boolean;
  cost: number;
  revenue: number;
}

function dayLabel(t: number): string {
  const d = new Date(t * 1000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

/** recharts tooltip 的 payload 是 unknown 形状，这里安全地读取 speedEst。 */
export function isSpeedEstimated(payload: unknown): boolean {
  return typeof payload === 'object' && payload !== null && 'speedEst' in payload && payload.speedEst === true;
}

export function toSeriesData(points: SeriesPoint[], bucket: StatsBucket): SeriesDatum[] {
  return points.map((p) => ({
    t: p.t,
    label: bucket === 'hour' ? formatHour(p.t) : dayLabel(p.t),
    requests: p.requests,
    rate: p.success_rate === null ? null : p.success_rate * 100,
    frt50: p.frt_p50,
    frt95: p.frt_p95,
    sp50: p.speed_p50,
    sp95: p.speed_p95,
    speedEst: p.speed_estimated,
    cost: p.cost_cny,
    revenue: p.revenue_cny,
  }));
}
