import type { NewapiPrice, Price } from '@/api/types';
import { formatSignificant } from '@/lib/format';

const DASH = '—';

interface PriceFields {
  input: number | null;
  output: number | null;
  cache_read: number | null;
  per_request: number | null;
  cache_write_5m?: number | null;
}

function Part({ label, value }: { label: string; value: number | null | undefined }) {
  return (
    <span className="whitespace-nowrap">
      <span className="text-xs text-muted-foreground">{label}</span> {formatSignificant(value ?? null)}
    </span>
  );
}

function PriceFieldsText({ price }: { price: PriceFields }) {
  if (price.per_request !== null && price.input === null && price.output === null) {
    return <Part label="按次" value={price.per_request} />;
  }
  if (price.input === null && price.output === null && price.cache_read === null) {
    return <span className="text-muted-foreground">{DASH}</span>;
  }
  return (
    <span className="inline-flex flex-wrap gap-x-2 tabular-nums">
      <Part label="入" value={price.input} />
      <Part label="出" value={price.output} />
      <Part label="读" value={price.cache_read} />
      {price.cache_write_5m !== undefined && <Part label="写" value={price.cache_write_5m} />}
    </span>
  );
}

/** 原厂价：输入 / 输出 / 缓存读 / 缓存写（美元 / 百万），或按次价（美元 / 次）。 */
export function OfficialPriceText({ price }: { price: Price | null }) {
  if (!price) return <span className="text-muted-foreground">{DASH}</span>;
  return <PriceFieldsText price={price} />;
}

/** NewAPI 当前价：输入 / 输出 / 缓存读，或按次价。 */
export function NewapiPriceText({ price }: { price: NewapiPrice | null }) {
  if (!price) return <span className="text-muted-foreground">{DASH}</span>;
  return <PriceFieldsText price={{ ...price, cache_write_5m: price.cache_write }} />;
}
