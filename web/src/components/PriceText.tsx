import type { CSSProperties } from 'react';
import type { NewapiPrice, Price } from '@/api/types';
import { WidthTiers } from '@/components/cells/WidthTiers';
import { DASH, formatSignificant } from '@/lib/format';
import {
  isEmptyPrice,
  priceSlots,
  priceTiers,
  priceTitle,
  SLOT_GAP_CH,
  SLOT_SPACE_CH,
  type PriceFieldsLike,
  type PriceSlot,
} from '@/lib/priceSlots';
import { cn } from '@/lib/utils';

const chStyle = (n: number): CSSProperties => ({ width: `${n}ch` });

/** 一个槽位：「数值 + 空格 + 标签」。数值在固定宽度里右对齐，缺值显示「—」，所以同一槽位的数值右边缘和标签上下对齐。 */
function Slot({ slot }: { slot: PriceSlot }) {
  return (
    <span className="flex shrink-0 items-baseline" style={chStyle(slot.ch)}>
      {/* justify-end：数值超宽时向左溢出，不压到标签 */}
      <span className="flex shrink-0 justify-end" style={chStyle(slot.valueCh)}>
        <span className={cn('shrink-0', slot.value === null && 'text-muted-foreground')}>{formatSignificant(slot.value)}</span>
      </span>
      <span className="shrink-0" style={{ ...chStyle(slot.labelCh + SLOT_SPACE_CH), paddingLeft: `${SLOT_SPACE_CH}ch` }}>
        <span className="text-xs text-muted-foreground">{slot.label}</span>
      </span>
    </span>
  );
}

function SlotRow({ slots }: { slots: ReadonlyArray<PriceSlot> }) {
  return (
    <div className="flex" style={{ columnGap: `${SLOT_GAP_CH}ch` }}>
      {slots.map((s) => (
        <Slot key={s.key} slot={s} />
      ))}
    </div>
  );
}

/**
 * 价格：固定槽位（入 / 出 / 读 / 写 / 写1h），按次价只显示「按次」。
 * 默认按列宽降级：全部槽位 → 入 / 出 → 只显示出（所有行同时切换）；full 时不降级（抽屉等宽度充足的地方）。
 */
function PriceFieldsText({ price, full }: { price: PriceFieldsLike; full?: boolean }) {
  if (isEmptyPrice(price)) return <span className="text-muted-foreground">{DASH}</span>;
  const slots = priceSlots(price);
  if (full) {
    return (
      <div className="whitespace-nowrap tabular-nums">
        <SlotRow slots={slots} />
      </div>
    );
  }
  const tiers = priceTiers(slots);
  return (
    <WidthTiers
      title={priceTitle(slots, (v) => formatSignificant(v))}
      tiers={tiers.map((t) => ({ min: `${t.ch}ch`, node: <SlotRow slots={t.slots} /> }))}
    />
  );
}

/** 价格库价格（原厂价）：入 / 出 / 读 / 写 / 写1h（$/M），或按次价（美元 / 次）。 */
export function OfficialPriceText({ price, full }: { price: Price | null; full?: boolean }) {
  if (!price) return <span className="text-muted-foreground">{DASH}</span>;
  return <PriceFieldsText price={price} full={full} />;
}

/** NewAPI 当前价：入 / 出 / 读 / 写（没有写1h），或按次价，排版与价格库价格一致。 */
export function NewapiPriceText({ price, full }: { price: NewapiPrice | null; full?: boolean }) {
  if (!price) return <span className="text-muted-foreground">{DASH}</span>;
  return <PriceFieldsText price={{ ...price, cache_write_5m: price.cache_write }} full={full} />;
}
