import type { CSSProperties } from 'react';

export interface CostRange {
  min: number;
  max: number;
}

/** 取一组成本值的范围；少于两个有效值或全部相同时返回 null（不上色）。 */
export function costRangeOf(values: Array<number | null>): CostRange | null {
  const valid = values.filter((v): v is number => v !== null && Number.isFinite(v));
  if (valid.length < 2) return null;
  const min = Math.min(...valid);
  const max = Math.max(...valid);
  return max > min ? { min, max } : null;
}

/** 便宜 → 绿，昂贵 → 红；透明度很低，只做淡淡的提示。 */
export function costStyle(value: number | null, range: CostRange | null): CSSProperties | undefined {
  if (value === null || range === null) return undefined;
  const ratio = Math.min(1, Math.max(0, (value - range.min) / (range.max - range.min)));
  const hue = 142 * (1 - ratio);
  return { backgroundColor: `hsl(${hue.toFixed(0)} 70% 45% / 0.16)`, borderRadius: 4, padding: '0 4px' };
}
