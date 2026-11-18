import type { CSSProperties, ReactNode } from 'react';

export interface WidthTier {
  /** 显示这一档至少需要的列宽，CSS 长度（如 `34ch`）。最后一档（最窄）的值不参与判断。 */
  min: string;
  node: ReactNode;
}

const minWidthLayer = (min: string): string => `calc((100cqw - ${min} + 1px) * 1000)`;
const maxWidthLayer = (upperMin: string): string => `calc((${upperMin} - 1px - 100cqw) * 1000)`;

function layerMaxWidth(tiers: ReadonlyArray<WidthTier>, i: number): CSSProperties['maxWidth'] {
  // 第 i 档：列宽 ≥ 自己的 min 且 < 上一档的 min 时显示
  const parts: string[] = [];
  if (i < tiers.length - 1) parts.push(minWidthLayer(tiers[i].min));
  if (i > 0) parts.push(maxWidthLayer(tiers[i - 1].min));
  if (parts.length === 0) return undefined;
  return parts.length === 1 ? parts[0] : `min(${parts.join(', ')})`;
}

/**
 * 按列宽自动降级的单元格：tiers 从宽到窄排列，列宽够哪一档就显示哪一档，所有行同时切换。
 * 做法同 BalanceCell：每一档的 max-width 用 CSS 容器查询单位 cqw 算，不选中的那档算成 0，
 * 所以不需要 JS 监听列宽。外层的占位块宽度等于最宽一档，让「自动调整列宽」测到完整排版的宽度。
 */
export function WidthTiers({ tiers, title }: { tiers: ReadonlyArray<WidthTier>; title?: string }) {
  if (tiers.length === 0) return null;
  return (
    <div className="relative whitespace-nowrap tabular-nums" title={title}>
      <div aria-hidden className="invisible overflow-hidden" style={{ width: tiers[0].min }}>
        &nbsp;
      </div>
      <div className="absolute inset-0" style={{ containerType: 'inline-size' }}>
        <div className="grid grid-cols-[minmax(0,1fr)]">
          {tiers.map((t, i) => (
            <div
              key={i}
              className="col-start-1 row-start-1 overflow-hidden"
              style={{ maxWidth: layerMaxWidth(tiers, i) }}
            >
              {t.node}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
