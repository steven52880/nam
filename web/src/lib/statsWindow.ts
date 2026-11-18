import type { StatsWindow } from '@/api/types';

/** 统计时长：后端接受 latest，或 <正整数>h / <正整数>d；上限 186 天（4464 小时）。 */
export type StatsWindowUnit = 'h' | 'd';

export const MAX_WINDOW_DAYS = 186;
export const MAX_WINDOW_HOURS = MAX_WINDOW_DAYS * 24;

const MAX_BY_UNIT: Record<StatsWindowUnit, number> = { h: MAX_WINDOW_HOURS, d: MAX_WINDOW_DAYS };

export interface CustomWindow {
  amount: number;
  unit: StatsWindowUnit;
}

/** 校验输入框文字：返回错误提示，合法返回 null。 */
export function validateWindowInput(text: string, unit: StatsWindowUnit): string | null {
  const trimmed = text.trim();
  if (trimmed === '') return '请输入时长';
  if (!/^\d+$/.test(trimmed)) return '请输入正整数';
  const amount = Number(trimmed);
  if (amount < 1) return '时长至少为 1';
  const max = MAX_BY_UNIT[unit];
  if (amount > max) return `最多 ${max} ${unit === 'h' ? '小时' : '天'}（${MAX_WINDOW_DAYS} 天）`;
  return null;
}

/** 由数值和单位拼出统计时长；不合法返回 null。 */
export function buildStatsWindow(amount: number, unit: StatsWindowUnit): StatsWindow | null {
  if (!Number.isInteger(amount) || amount < 1 || amount > MAX_BY_UNIT[unit]) return null;
  return `${amount}${unit}`;
}

/** 解析 12h / 30d；latest、不合法的值返回 null。 */
export function parseCustomWindow(value: string): CustomWindow | null {
  const m = /^([1-9]\d*)([hd])$/.exec(value);
  if (!m) return null;
  const amount = Number(m[1]);
  const unit = m[2] as StatsWindowUnit;
  return amount <= MAX_BY_UNIT[unit] ? { amount, unit } : null;
}

/** 下拉里固定显示的几项；其余值都算自定义。 */
export const PRESET_WINDOWS = ['latest', '3d', '7d'] as const satisfies readonly StatsWindow[];

export function isPresetWindow(value: string): boolean {
  return (PRESET_WINDOWS as readonly string[]).includes(value);
}
