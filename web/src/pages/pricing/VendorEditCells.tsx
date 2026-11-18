import { useRef, useState, type ReactNode } from 'react';
import type { VendorPricingItem, VendorPricingMode } from '@/api/types';
import { Combobox, type ComboboxItem } from '@/components/ui/combobox';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { DASH, formatGroupRatio, formatNumber, formatRatioPercent, roundGroupRatio } from '@/lib/format';
import { PRICING_MODE } from '@/lib/labels';
import {
  cnyPerUsd,
  currentPrice,
  groupRatioToText,
  parseGroupRatioInput,
  parseMarkupInput,
  parseSellPriceInput,
  ratioToPercentText,
  type ParseResult,
} from '@/lib/vendorPricing';
import { cn } from '@/lib/utils';

/** 点一下变成输入框；回车提交，Esc 取消，点别处时输入合法且有改动就提交。 */
function InlineEdit({
  display,
  initial,
  suffix,
  parse,
  onSave,
  title,
  disabled,
}: {
  display: ReactNode;
  initial: string;
  suffix?: string;
  parse: (text: string) => ParseResult;
  onSave: (value: number) => void;
  title?: string;
  disabled?: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(initial);
  const [error, setError] = useState('');
  // 回车或 Esc 已经处理过时，随后的失焦不再重复提交
  const done = useRef(false);

  const start = () => {
    done.current = false;
    setText(initial);
    setError('');
    setEditing(true);
  };
  const finish = (submit: boolean) => {
    if (done.current) return;
    if (!submit || text.trim() === initial) {
      done.current = true;
      setEditing(false);
      return;
    }
    const r = parse(text);
    if (!r.ok) {
      setError(r.error);
      return;
    }
    done.current = true;
    setEditing(false);
    onSave(r.value);
  };

  if (!editing) {
    return (
      <button
        type="button"
        data-no-row-click=""
        disabled={disabled}
        title={title ?? '点击修改'}
        onClick={start}
        className="inline-flex max-w-full items-center rounded px-1 text-left hover:bg-accent/60 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {display}
      </button>
    );
  }
  return (
    <span className="inline-flex items-center gap-1" data-no-row-click="">
      <input
        autoFocus
        value={text}
        inputMode="decimal"
        aria-invalid={error ? true : undefined}
        title={error || undefined}
        onFocus={(e) => e.currentTarget.select()}
        onChange={(e) => {
          setText(e.target.value);
          setError('');
        }}
        onBlur={() => {
          if (!error) finish(true);
          else {
            done.current = true;
            setEditing(false);
          }
        }}
        onKeyDown={(e) => {
          e.stopPropagation();
          if (e.key === 'Enter') {
            e.preventDefault();
            finish(true);
          } else if (e.key === 'Escape') {
            e.preventDefault();
            finish(false);
          }
        }}
        className={cn(
          'h-6 w-20 rounded border bg-background px-1 text-right text-base tabular-nums outline-none focus-visible:ring-1 focus-visible:ring-ring',
          error ? 'border-destructive' : 'border-input',
        )}
      />
      {suffix && <span className="text-muted-foreground">{suffix}</span>}
      {error && <span className="text-xs text-destructive">{error}</span>}
    </span>
  );
}

const MODE_ITEMS: ComboboxItem[] = (['fixed', 'follow'] as const).map((m) => ({ value: m, label: PRICING_MODE[m].label }));

export function ModeCell({ row, onChange }: { row: VendorPricingItem; onChange: (row: VendorPricingItem, mode: VendorPricingMode) => void }) {
  return (
    <Combobox
      variant="cell"
      searchable={false}
      items={MODE_ITEMS}
      value={row.mode}
      title="固定 = 手填售价；跟随 = 参考成本 × 加价比例"
      onChange={(v) => {
        if (v && v !== row.mode) onChange(row, v === 'follow' ? 'follow' : 'fixed');
      }}
    />
  );
}

export function MarkupCell({ row, onSave }: { row: VendorPricingItem; onSave: (row: VendorPricingItem, markup: number) => void }) {
  if (row.mode !== 'follow') return <span className="text-muted-foreground" title="只有跟随模式用加价比例">{DASH}</span>;
  const text = row.markup === null ? DASH : `× ${formatNumber(row.markup, 4, { trim: true })}`;
  return (
    <InlineEdit
      display={<span className="tabular-nums">{text}</span>}
      initial={row.markup === null ? '' : String(row.markup)}
      parse={parseMarkupInput}
      onSave={(v) => onSave(row, v)}
      title="点击修改加价比例；1.5 表示在参考成本上加价 50%"
    />
  );
}

/** 折合人民币小字：数字右对齐、定宽，所有行的「≈ / 元/原厂 $1」上下对齐。 */
function CnyHint({ price, usdRate }: { price: number; usdRate: number }) {
  return (
    <span className="ml-1.5 inline-flex text-xs text-muted-foreground tabular-nums">
      <span>≈&nbsp;</span>
      <span className="text-right" style={{ width: '5ch' }}>{formatNumber(cnyPerUsd(price, usdRate), 2)}</span>
      <span>&nbsp;元/原厂 $1</span>
    </span>
  );
}

export function SellPriceCell({
  row,
  usdRate,
  onSave,
}: {
  row: VendorPricingItem;
  usdRate: number;
  onSave: (row: VendorPricingItem, price: number) => void;
}) {
  const price = currentPrice(row);
  const shown = (
    <>
      <span className="inline-block text-right tabular-nums" style={{ width: '6ch' }}>{formatRatioPercent(price)}</span>
      {price !== null && <CnyHint price={price} usdRate={usdRate} />}
    </>
  );
  if (row.mode === 'follow') {
    return (
      <span className="px-1" title="跟随模式：售价 = 参考成本 × 加价比例，不能直接改">
        {shown}
      </span>
    );
  }
  return (
    <InlineEdit
      display={price === null ? <span className="text-muted-foreground">点击设置</span> : shown}
      initial={row.sell_price === null ? '' : ratioToPercentText(row.sell_price)}
      suffix="%"
      parse={parseSellPriceInput}
      onSave={(v) => onSave(row, v)}
      title="点击修改售价（原厂价的百分比）"
    />
  );
}

/** 目标倍率：固定模式点击直接修改（改完后后端把倍率反算成售价），跟随模式只读。 */
export function GroupRatioCell({
  row,
  onSave,
}: {
  row: VendorPricingItem;
  onSave: (row: VendorPricingItem, ratio: number) => void;
}) {
  const ratio = row.target_ratio;
  const shown = <span className="tabular-nums">{formatGroupRatio(ratio)}</span>;
  if (row.mode === 'follow') {
    return (
      <span className="px-1" title="跟随模式的倍率由参考成本决定，改成固定模式后可以直接编辑">
        {shown}
      </span>
    );
  }
  return (
    <InlineEdit
      display={ratio === null ? <span className="text-muted-foreground">点击设置</span> : shown}
      initial={ratio === null ? '' : groupRatioToText(ratio)}
      parse={parseGroupRatioInput}
      onSave={(v) => {
        if (ratio === null || v !== roundGroupRatio(ratio)) onSave(row, v);
      }}
      title="点击直接修改倍率（保留 3 位小数），改完后售价按倍率反算"
    />
  );
}

export function ReferenceCostCell({ row }: { row: VendorPricingItem }) {
  const ref = row.reference;
  if (!ref) return <span className="text-muted-foreground">{DASH}</span>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="cursor-default tabular-nums">{formatRatioPercent(ref.cost)}</span>
      </TooltipTrigger>
      <TooltipContent className="max-w-md">
        <div className="mb-0.5 text-muted-foreground">优先级 {ref.priority}，按权重加权</div>
        <ul>
          {ref.channels.map((c) => (
            <li key={c.id} className="flex gap-2 tabular-nums">
              <span className="min-w-0 flex-1 truncate">{c.name}</span>
              <span className="text-muted-foreground">权重 {c.weight}</span>
              <span>{formatRatioPercent(c.channel_cost)}</span>
            </li>
          ))}
        </ul>
      </TooltipContent>
    </Tooltip>
  );
}
