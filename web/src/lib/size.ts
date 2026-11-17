let cachedRootPx: number | null = null;

/** 根字号（像素）。第一次调用时读取并缓存；没有 DOM 时回退 16。 */
export function rootPx(): number {
  if (cachedRootPx !== null) return cachedRootPx;
  if (typeof document === 'undefined' || typeof getComputedStyle !== 'function') return 16;
  const v = parseFloat(getComputedStyle(document.documentElement).fontSize);
  cachedRootPx = Number.isFinite(v) && v > 0 ? v : 16;
  return cachedRootPx;
}

export function remToPx(rem: number): number {
  return rem * rootPx();
}

export function pxToRem(px: number): number {
  return px / rootPx();
}

/** 保存列宽时取到 0.25rem，避免浮点噪声导致"未保存"误报。 */
export function roundRem(rem: number): number {
  return Math.round(rem * 4) / 4;
}
