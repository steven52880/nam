import type { CSSProperties } from 'react';
import { BALANCE_SEP_CH, balanceFullCh, balanceParts, balanceTitle, type BalanceWidths } from '@/lib/balance';
import { DASH } from '@/lib/format';
import { Dash } from './Dash';

const chStyle = (n: number): CSSProperties => ({ width: `${n}ch` });

/**
 * 余额：「$12.30 ≈ ¥88.10」，两者都为空时显示灰色「—」。
 *
 * - 传了 widths（列级最大字符数）：每一行都按同样的定宽排版，所有 ≈ 上下对齐。
 * - 列宽放不下最宽那一行的「美元 ≈ 人民币」时，改为只显示人民币（没有人民币的行显示美元）。
 *   切换用 CSS 容器查询单位 cqw 做：宽的那份 max-width 在列宽不够时算成 0，窄的那份反之，
 *   所以不需要 JS 监听列宽，拖列宽时单元格也不会重新渲染。
 * - 外层的占位块让「自动调整列宽」测到的是完整排版的宽度（容器查询的容器本身没有固有宽度）。
 */
export function BalanceCell({ usd, cny, widths }: { usd: number | null; cny: number | null; widths?: BalanceWidths }) {
  if (usd === null && cny === null) return <Dash />;
  const parts = balanceParts(usd, cny);
  const w: BalanceWidths = widths ?? {
    usdCh: parts.usd?.length ?? DASH.length,
    cnyCh: parts.cny?.length ?? 0,
  };
  const full = balanceFullCh(w);
  const narrowText = parts.cny ?? parts.usd;
  return (
    <div className="relative whitespace-nowrap tabular-nums" title={balanceTitle(parts)}>
      <div aria-hidden className="invisible overflow-hidden" style={chStyle(full)}>
        &nbsp;
      </div>
      <div className="absolute inset-0" style={{ containerType: 'inline-size' }}>
        <div className="grid grid-cols-[minmax(0,1fr)]">
          {/* 完整排版：美元右对齐到 ≈ 左边，人民币左对齐到 ≈ 右边；容器宽度 ≥ full 才显示 */}
          <div
            className="col-start-1 row-start-1 flex justify-end overflow-hidden"
            style={{ maxWidth: `calc((100cqw - ${full}ch + 1px) * 1000)` }}
          >
            <span className="shrink-0 text-right" style={chStyle(w.usdCh)}>{parts.usd ?? DASH}</span>
            {w.cnyCh > 0 && (
              <>
                <span className="shrink-0 text-center text-muted-foreground" style={chStyle(BALANCE_SEP_CH)}>
                  {parts.cny === null ? '' : '≈'}
                </span>
                <span className="shrink-0 text-left text-muted-foreground" style={chStyle(w.cnyCh)}>{parts.cny ?? ''}</span>
              </>
            )}
          </div>
          {/* 窄排版：只显示人民币；容器宽度 < full 才显示 */}
          <div
            className="col-start-1 row-start-1 overflow-hidden text-right"
            style={{ maxWidth: `calc((${full}ch - 1px - 100cqw) * 1000)` }}
          >
            {narrowText}
          </div>
        </div>
      </div>
    </div>
  );
}
