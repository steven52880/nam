import type { VendorPricingItem, VendorPricingMode } from '@/api/types';
import { customCol, numberCol, textCol, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { DASH, formatGroupRatio, formatNumber, formatPercent, formatTime } from '@/lib/format';
import { labelOf, PRICING_MODE, VENDOR_PRICING_STATE } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { currentPrice, rawRatioTitle, signedChange } from '@/lib/vendorPricing';
import { GroupRatioCell, MarkupCell, ModeCell, ReferenceCostCell, SellPriceCell } from './VendorEditCells';

export interface VendorRowActions {
  usdRate: number;
  /** 正在提交的厂商，按钮置灰。 */
  busyId: number | null | undefined;
  onMode: (row: VendorPricingItem, mode: VendorPricingMode) => void;
  onMarkup: (row: VendorPricingItem, markup: number) => void;
  onSellPrice: (row: VendorPricingItem, price: number) => void;
  onGroupRatio: (row: VendorPricingItem, ratio: number) => void;
  onConfirm: (row: VendorPricingItem) => void;
  onAdopt: (row: VendorPricingItem) => void;
  onOverride: (row: VendorPricingItem) => void;
}

export const SMALL_CHANGE_NOTE = '变化 < 最小变动，自动写入会跳过';

/** 倍率三段式各段宽度（ch）：当前倍率右对齐、箭头居中、目标倍率左对齐，所有厂商的箭头上下对齐。 */
const RATIO_CURRENT_CH = 5;
const RATIO_ARROW_CH = 2.5;
const RATIO_TARGET_CH = 5.5;
/** 三段式完整排版需要的列宽；目标倍率按钮自带 0.5rem 内边距。 */
const RATIO_FULL = `calc(${RATIO_CURRENT_CH + RATIO_ARROW_CH + RATIO_TARGET_CH}ch + 0.5rem)`;

/**
 * NewAPI 当前倍率 → 目标倍率；固定模式下目标倍率可以直接点击修改。
 * 列宽够放三段式时全部显示；不够时所有行都只显示目标倍率（当前倍率和箭头折叠成 0 宽）。
 * 切换用 CSS 容器查询单位 cqw（同 BalanceCell），目标倍率只渲染一份，两种形态下都能编辑。
 */
function RatioCell({ row, onSave }: { row: VendorPricingItem; onSave: (row: VendorPricingItem, ratio: number) => void }) {
  return (
    <div className="relative whitespace-nowrap tabular-nums">
      <div aria-hidden className="invisible overflow-hidden" style={{ width: RATIO_FULL }}>
        &nbsp;
      </div>
      <div className="absolute inset-0" style={{ containerType: 'inline-size' }}>
        <div className="flex items-center">
          <div
            className="flex shrink-0 overflow-hidden"
            style={{ maxWidth: `calc((100cqw - ${RATIO_FULL} + 1px) * 1000)` }}
          >
            <span
              className={cn(
                'shrink-0 text-right',
                row.newapi_ratio === null && 'text-muted-foreground',
                row.state === 'drift' && 'font-medium text-destructive',
              )}
              style={{ width: `${RATIO_CURRENT_CH}ch` }}
              title={row.newapi_ratio === null ? 'NewAPI 里还没有这个分组' : `NewAPI 当前倍率，${rawRatioTitle(row.newapi_ratio)}`}
            >
              {formatGroupRatio(row.newapi_ratio)}
            </span>
            <span className="shrink-0 text-center text-muted-foreground" style={{ width: `${RATIO_ARROW_CH}ch` }}>→</span>
          </div>
          <GroupRatioCell row={row} onSave={onSave} />
        </div>
      </div>
    </div>
  );
}

function ChangeCell({ row }: { row: VendorPricingItem }) {
  const v = signedChange(row);
  if (v === null) return <span className="text-muted-foreground">{DASH}</span>;
  return (
    <span className={cn('tabular-nums', row.state === 'pending' && 'font-medium text-warning')}>
      {formatPercent(v, { signed: true })}
    </span>
  );
}

function MarginCell({ row }: { row: VendorPricingItem }) {
  if (row.margin_7d === null) return <span className="text-muted-foreground">{DASH}</span>;
  const title = `近 7 天 收入 ¥${formatNumber(row.revenue_7d, 2)}，成本 ¥${formatNumber(row.cost_7d, 2)}`;
  return (
    <span className={cn('tabular-nums', row.margin_7d < 0 && 'text-destructive')} title={title}>
      {formatPercent(row.margin_7d)}
    </span>
  );
}

function StateCell({ row }: { row: VendorPricingItem }) {
  const def = labelOf(VENDOR_PRICING_STATE, row.state);
  const small = row.state === 'to_write' && row.small_change;
  let title = def.hint;
  if (row.state === 'pending' && row.pending_price !== null) title = `${def.hint}（待写入售价见「变化」列）`;
  if (small) title = SMALL_CHANGE_NOTE;
  return (
    <span className="inline-flex items-center gap-1.5">
      <Badge tone={def.tone} title={title}>{def.label}</Badge>
      {small && <span className="text-xs text-muted-foreground" title={SMALL_CHANGE_NOTE}>变化 &lt; 最小变动</span>}
    </span>
  );
}

function ActionCell({ row, a }: { row: VendorPricingItem; a: VendorRowActions }) {
  const busy = a.busyId === row.vendor_id;
  if (row.state === 'pending') {
    return <Button size="sm" variant="outline" disabled={busy} onClick={() => a.onConfirm(row)}>确认</Button>;
  }
  if (row.state === 'drift') {
    return (
      <span className="inline-flex items-center gap-1">
        <Button size="sm" variant="outline" disabled={busy} onClick={() => a.onAdopt(row)} title="转固定模式，售价按 NewAPI 当前倍率反算，只改 nam">采纳 NewAPI 值</Button>
        <Button size="sm" variant="outline" className="text-destructive" disabled={busy} onClick={() => a.onOverride(row)} title="把 NewAPI 的倍率改回 nam 的目标倍率">以 nam 为准覆盖</Button>
      </span>
    );
  }
  return null;
}

export const HEADER_HINTS = {
  mode: '固定 = 手填售价；跟随 = 参考成本 × 加价比例。',
  markup: '跟随模式：售价 = 参考成本 × 加价比例。例：参考成本 20%、加价比例 1.5，售价 = 30%。',
  reference: '参考成本 = 该厂商分组下最高优先级一层启用渠道的上游渠道售价，按权重加权（权重全为 0 时取平均）。单位是原厂价的百分比；悬停数值可看用了哪些渠道。',
  sell: '售价是原厂价的百分比。固定 = 手填；跟随 = 参考成本 × 加价比例。小字是折合人民币：售价 × 美元参考汇率。例：30% × 7.2 ≈ 2.16 元 / 原厂 $1。',
  ratio: '厂商分组倍率 = 售价 × 美元参考汇率 ÷ 站点汇率，保留 3 位小数。例：30% × 7.2 ÷ 7 ≈ 0.309。左边是 NewAPI 当前值，右边是目标值；列太窄时只显示目标值。固定模式下倍率可以直接改，改完后售价按倍率反算。',
  change: '变化 = (目标售价 − 上次写入售价) ÷ 上次写入售价。超过变化保护阈值要先确认；小于最小变动时自动写入会跳过。',
  margin: '毛利率 = 近 7 天（收入 − 成本）÷ 收入。例：收入 100 元、成本 70 元，毛利率 30%。',
} as const;

/** 厂商定价表的列。actions 要保持引用稳定（useMemo）。 */
export function vendorColumns(a: VendorRowActions): TableColumn<VendorPricingItem>[] {
  return [
    textCol<VendorPricingItem>('vendor', '厂商', (r) => r.vendor, {
      width: 10,
      cell: (r) => (
        <span className="inline-flex items-baseline gap-1.5">
          <span className="font-medium">{r.vendor}</span>
          <span className="text-xs text-muted-foreground">{r.group}</span>
        </span>
      ),
    }),
    textCol<VendorPricingItem>('mode', '模式', (r) => labelOf(PRICING_MODE, r.mode).label, {
      width: 6.5,
      headerTitle: HEADER_HINTS.mode,
      cell: (r) => <ModeCell row={r} onChange={a.onMode} />,
    }),
    numberCol<VendorPricingItem>('markup', '加价比例', (r) => (r.mode === 'follow' ? r.markup : null), {
      width: 7,
      headerTitle: HEADER_HINTS.markup,
      cell: (r) => <MarkupCell row={r} onSave={a.onMarkup} />,
    }),
    numberCol<VendorPricingItem>('reference', '参考成本', (r) => r.reference?.cost ?? null, {
      width: 7,
      headerTitle: HEADER_HINTS.reference,
      cell: (r) => <ReferenceCostCell row={r} />,
    }),
    numberCol<VendorPricingItem>('sell_price', '售价', (r) => currentPrice(r), {
      width: 17,
      align: 'left',
      headerTitle: HEADER_HINTS.sell,
      cell: (r) => <SellPriceCell row={r} usdRate={a.usdRate} onSave={a.onSellPrice} />,
    }),
    numberCol<VendorPricingItem>('ratio', '厂商分组倍率', (r) => r.target_ratio, {
      width: 13,
      align: 'left',
      headerTitle: HEADER_HINTS.ratio,
      cell: (r) => <RatioCell row={r} onSave={a.onGroupRatio} />,
    }),
    numberCol<VendorPricingItem>('change', '变化', (r) => signedChange(r), {
      width: 6,
      headerTitle: HEADER_HINTS.change,
      cell: (r) => <ChangeCell row={r} />,
    }),
    numberCol<VendorPricingItem>('margin', '毛利率', (r) => r.margin_7d, {
      width: 6,
      headerTitle: HEADER_HINTS.margin,
      cell: (r) => <MarginCell row={r} />,
    }),
    textCol<VendorPricingItem>('state', '状态', (r) => labelOf(VENDOR_PRICING_STATE, r.state).label, {
      width: 12,
      cell: (r) => <StateCell row={r} />,
    }),
    textCol<VendorPricingItem>('written_at', '上次写入', (r) => formatTime(r.last_written_at ?? 0), {
      width: 10,
      hidden: true,
    }),
    customCol<VendorPricingItem>('actions', '', {
      width: 16,
      align: 'right',
      fixed: true,
      cell: (r) => <ActionCell row={r} a={a} />,
    }),
  ];
}
