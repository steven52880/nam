/**
 * 价格列的固定槽位：入 / 出 / 读 / 写 / 写1h。每个槽位的宽度固定（单位 ch），
 * 所以不同行、不同列的同一槽位可以上下对齐；值缺失时显示「—」占位。
 * 列宽不够时按档位降级：全部槽位 → 入 / 出 → 只显示出。
 */

export type PriceSlotKey = 'input' | 'output' | 'cache_read' | 'cache_write_5m' | 'cache_write_1h' | 'per_request';

/** 价格字段；cache_write_5m / cache_write_1h 为 undefined 表示这份数据没有对应槽位。 */
export interface PriceFieldsLike {
  input: number | null;
  output: number | null;
  cache_read: number | null;
  per_request: number | null;
  cache_write_5m?: number | null;
  cache_write_1h?: number | null;
}

export interface PriceSlot {
  key: PriceSlotKey;
  label: string;
  value: number | null;
  /** 数值部分宽度（ch），数值在里面右对齐。 */
  valueCh: number;
  /** 标签文字宽度（ch，标签用小一号字，所以比数值字符窄）。 */
  labelCh: number;
  /** 整个槽位宽度（ch）= 数值 + 空格 + 标签。 */
  ch: number;
}

/** 数值和标签之间的空格宽度（ch）。 */
export const SLOT_SPACE_CH = 0.5;
/** 槽位之间的间距（ch）。 */
export const SLOT_GAP_CH = 1;

/**
 * 排版是「数值 + 空格 + 标签」，如 `3 入  15 出  0.3 读`。数值宽度按常见最大值留：
 * 入 / 出 / 写1h 常见 ≤ 4 个字符（12.5、3.75），读 / 写常见到 5 个字符（0.075）。
 * 数值超宽时向左溢出（不会压到标签）。
 */
const LAYOUT: Record<PriceSlotKey, { label: string; valueCh: number; labelCh: number }> = {
  input: { label: '入', valueCh: 4.5, labelCh: 1.6 },
  output: { label: '出', valueCh: 4.5, labelCh: 1.6 },
  cache_read: { label: '读', valueCh: 5, labelCh: 1.6 },
  cache_write_5m: { label: '写', valueCh: 5, labelCh: 1.6 },
  cache_write_1h: { label: '写1h', valueCh: 4.5, labelCh: 3.2 },
  per_request: { label: '按次', valueCh: 4.5, labelCh: 3.2 },
};

function slot(key: PriceSlotKey, value: number | null | undefined): PriceSlot {
  const { label, valueCh, labelCh } = LAYOUT[key];
  return { key, label, value: value ?? null, valueCh, labelCh, ch: valueCh + SLOT_SPACE_CH + labelCh };
}

/** 按次价：只有「按次」一个槽位，放在第一个槽位的位置。 */
export function isPerRequestPrice(p: PriceFieldsLike): boolean {
  return p.per_request !== null && p.input === null && p.output === null;
}

/** 完全没有价格（既不是按次价，也没有入 / 出 / 读）。 */
export function isEmptyPrice(p: PriceFieldsLike): boolean {
  return !isPerRequestPrice(p) && p.input === null && p.output === null && p.cache_read === null;
}

/** 这份价格的全部槽位：入 / 出 / 读 必有，写 / 写1h 只在数据里有该字段时才有；按次价只有「按次」。 */
export function priceSlots(p: PriceFieldsLike): PriceSlot[] {
  if (isPerRequestPrice(p)) return [slot('per_request', p.per_request)];
  const slots = [slot('input', p.input), slot('output', p.output), slot('cache_read', p.cache_read)];
  if (p.cache_write_5m !== undefined) slots.push(slot('cache_write_5m', p.cache_write_5m));
  if (p.cache_write_1h !== undefined) slots.push(slot('cache_write_1h', p.cache_write_1h));
  return slots;
}

/** 一排槽位占的宽度（ch）：槽位宽度之和加上槽位间距。 */
export function slotsWidthCh(slots: ReadonlyArray<PriceSlot>): number {
  if (slots.length === 0) return 0;
  return slots.reduce((sum, s) => sum + s.ch, 0) + SLOT_GAP_CH * (slots.length - 1);
}

export interface PriceTier {
  slots: PriceSlot[];
  /** 显示这一档至少需要的列宽（ch）。 */
  ch: number;
}

/**
 * 降级档位，从宽到窄：全部槽位 → 入 / 出 → 只显示出。
 * 全部槽位不比「入 / 出」多（只有入、出两个槽位）或「入 / 出」就是「出」时，相同的档位会合并。
 */
export function priceTiers(slots: ReadonlyArray<PriceSlot>): PriceTier[] {
  const pick = (keys: PriceSlotKey[]) => slots.filter((s) => keys.includes(s.key));
  const candidates = [[...slots], pick(['input', 'output']), pick(['output'])];
  const tiers: PriceTier[] = [];
  for (const c of candidates) {
    if (c.length === 0) continue;
    if (tiers.length > 0 && tiers[tiers.length - 1].slots.length === c.length) continue;
    tiers.push({ slots: c, ch: slotsWidthCh(c) });
  }
  return tiers;
}

/** 悬停提示：把所有有值的槽位写成「入 3 / 出 15 / 读 0.3」；窄列只显示一部分时用来看全部。 */
export function priceTitle(slots: ReadonlyArray<PriceSlot>, format: (v: number) => string): string {
  return slots
    .filter((s) => s.value !== null)
    .map((s) => `${s.label} ${format(s.value as number)}`)
    .join(' / ');
}
