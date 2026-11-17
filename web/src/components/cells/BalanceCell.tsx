import { DASH, formatNumber } from '@/lib/format';
import { Dash } from './Dash';

/** 余额：「$12.30  ≈ ¥88.10」，两者都为空时显示灰色「—」。 */
export function BalanceCell({ usd, cny }: { usd: number | null; cny: number | null }) {
  if (usd === null && cny === null) return <Dash />;
  return (
    <span className="tabular-nums">
      {usd === null ? DASH : `$${formatNumber(usd)}`}
      <span className="ml-1 text-xs text-muted-foreground">
        {cny === null ? '' : `≈ ¥${formatNumber(cny)}`}
      </span>
    </span>
  );
}
