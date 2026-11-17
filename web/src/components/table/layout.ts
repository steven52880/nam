import { pxToRem, remToPx, roundRem } from '@/lib/size';
import type { SortSpec, TableColumn, TableLayout } from './types';

/** 默认列宽（rem）。 */
export const DEFAULT_WIDTH_REM = 6.25;
/** 默认最小列宽（rem）。 */
export const DEFAULT_MIN_REM = 3;
/** 列宽上限（rem）。 */
export const MAX_WIDTH_REM = 60;

export function defaultTableLayout(): TableLayout {
  return { widths: {}, order: [], visibility: {}, sort: null };
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

/**
 * 校验槽位里的原始数据。缺少的字段取默认值；字段存在但类型不对时整体返回 null（改用默认布局）。
 * 返回的是新对象，不与入参共享引用。
 */
export function parseTableLayout(data: unknown): TableLayout | null {
  if (!isRecord(data)) return null;
  const out = defaultTableLayout();

  if (data.widths !== undefined) {
    if (!isRecord(data.widths)) return null;
    for (const [id, w] of Object.entries(data.widths)) {
      if (typeof w !== 'number' || !Number.isFinite(w) || w <= 0) return null;
      out.widths[id] = w;
    }
  }
  if (data.order !== undefined) {
    if (!Array.isArray(data.order)) return null;
    for (const id of data.order) {
      if (typeof id !== 'string') return null;
      out.order.push(id);
    }
  }
  if (data.visibility !== undefined) {
    if (!isRecord(data.visibility)) return null;
    for (const [id, v] of Object.entries(data.visibility)) {
      if (typeof v !== 'boolean') return null;
      out.visibility[id] = v;
    }
  }
  if (data.sort !== undefined && data.sort !== null) {
    if (!isRecord(data.sort)) return null;
    const { id, desc } = data.sort;
    if (typeof id !== 'string' || typeof desc !== 'boolean') return null;
    const sort: SortSpec = { id, desc };
    out.sort = sort;
  }
  return out;
}

/**
 * 对账列顺序。三个参数都不含 fixed 列。
 * - 保留 saved 中仍存在于 currentIds 的 id（重复的只留第一个）。
 * - 新列（在 currentIds 里、不在 saved 里）按它们在 defaultIds 里的先后依次插入：
 *   放到默认顺序里"前一列"后面；前一列不在结果里时继续往前找最近的一列；都找不到就放最前。
 * - 不在 defaultIds 里的新列追加到末尾（按 currentIds 的顺序）。
 */
export function reconcileOrder(saved: string[], currentIds: string[], defaultIds: string[]): string[] {
  const exists = new Set(currentIds);
  const result: string[] = [];
  const inResult = new Set<string>();
  for (const id of saved) {
    if (exists.has(id) && !inResult.has(id)) {
      result.push(id);
      inResult.add(id);
    }
  }

  const defaults = defaultIds.filter((id) => exists.has(id));
  defaults.forEach((id, idx) => {
    if (inResult.has(id)) return;
    let at = 0;
    for (let p = idx - 1; p >= 0; p--) {
      const prevAt = result.indexOf(defaults[p]);
      if (prevAt >= 0) {
        at = prevAt + 1;
        break;
      }
    }
    result.splice(at, 0, id);
    inResult.add(id);
  });

  for (const id of currentIds) {
    if (!inResult.has(id)) {
      result.push(id);
      inResult.add(id);
    }
  }
  return result;
}

function minRemOf<T>(col: TableColumn<T>): number {
  return col.minWidth ?? DEFAULT_MIN_REM;
}

function clampRem<T>(col: TableColumn<T>, rem: number): number {
  const min = minRemOf(col);
  return Math.min(Math.max(rem, min), Math.max(min, MAX_WIDTH_REM));
}

function defaultRemOf<T>(col: TableColumn<T>): number {
  return clampRem(col, col.width ?? DEFAULT_WIDTH_REM);
}

/**
 * 每列的生效宽度（像素）：已保存的（rem）换算后限制在"最小列宽 ~ 60rem"，
 * 没保存的用默认宽度（同样限制）。结果包含所有列（含 fixed）。
 */
export function reconcileWidths<T>(saved: Record<string, number>, columns: TableColumn<T>[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const col of columns) {
    const s = saved[col.id];
    const rem = typeof s === 'number' && Number.isFinite(s) ? clampRem(col, s) : defaultRemOf(col);
    out[col.id] = remToPx(rem);
  }
  return out;
}

/** 向上取到 0.25rem；留一点容差，避免 13.75 因浮点噪声被抬到 14。 */
export function ceilRem(rem: number): number {
  return Math.ceil(rem * 4 - 1e-6) / 4;
}

/**
 * 把当前列宽（像素）转成要保存的形式：rem，取到 0.25，只保留与默认宽度不同的列。
 * fixed 列和不存在的列不记录。
 * 默认四舍五入（手动拖宽）；`ceilIds` 里的列向上取（自动调整：保存后的宽度不能小于测量值，
 * 否则会截断内容），上限仍是 60rem。
 */
export function diffWidths<T>(
  currentPx: Record<string, number>,
  columns: TableColumn<T>[],
  opts: { ceilIds?: ReadonlySet<string> } = {},
): Record<string, number> {
  const out: Record<string, number> = {};
  for (const col of columns) {
    if (col.fixed) continue;
    const px = currentPx[col.id];
    if (typeof px !== 'number' || !Number.isFinite(px)) continue;
    const clamped = clampRem(col, pxToRem(px));
    const rem = opts.ceilIds?.has(col.id) ? Math.min(ceilRem(clamped), MAX_WIDTH_REM) : roundRem(clamped);
    if (rem !== roundRem(defaultRemOf(col))) out[col.id] = rem;
  }
  return out;
}

/** 每列是否可见：fixed 列永远可见；其它列取已保存的值，没有则取默认（hidden 取反）。 */
export function effectiveVisibility<T>(saved: Record<string, boolean>, columns: TableColumn<T>[]): Record<string, boolean> {
  const out: Record<string, boolean> = {};
  for (const col of columns) {
    if (col.fixed) {
      out[col.id] = true;
      continue;
    }
    const s = saved[col.id];
    out[col.id] = typeof s === 'boolean' ? s : !col.hidden;
  }
  return out;
}

/** 只返回与默认不同的项；fixed 列和不存在的列不记录。 */
export function diffVisibility<T>(current: Record<string, boolean>, columns: TableColumn<T>[]): Record<string, boolean> {
  const out: Record<string, boolean> = {};
  for (const col of columns) {
    if (col.fixed) continue;
    const v = current[col.id];
    if (typeof v !== 'boolean') continue;
    if (v !== !col.hidden) out[col.id] = v;
  }
  return out;
}
