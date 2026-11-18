import { DescRow } from '@/components/ui/section';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/misc';
import { MAX_USD_CNY_RATE } from '@/lib/usdRate';
import { parseMinChange, parseThreshold, type ParseResult } from '@/lib/vendorPricing';

function PercentInput({ value, onChange, result }: { value: string; onChange: (v: string) => void; result: ParseResult }) {
  return (
    <span className="inline-flex items-center gap-1">
      <Input
        className="w-20 text-right tabular-nums"
        inputMode="decimal"
        value={value}
        aria-invalid={result.ok ? undefined : true}
        onChange={(e) => onChange(e.target.value)}
      />
      %
    </span>
  );
}

interface PricingFieldsProps {
  threshold: string;
  minChange: string;
  usdRate: string;
  usdRateOk: boolean;
  autoWrite: boolean;
  onThreshold: (v: string) => void;
  onMinChange: (v: string) => void;
  onUsdRate: (v: string) => void;
  onAutoWrite: (v: boolean) => void;
}

/** 设置页「定价参数」：变化保护阈值、最小变动、美元参考汇率、自动写入。 */
export function PricingFields(p: PricingFieldsProps) {
  const thresholdResult = parseThreshold(p.threshold);
  const minChangeResult = parseMinChange(p.minChange);
  return (
    <>
      <DescRow label="变化保护阈值">
        <span className="flex items-center gap-2">
          <PercentInput value={p.threshold} onChange={p.onThreshold} result={thresholdResult} />
          <span className="text-sm text-muted-foreground">跟随模式的售价变化超过它时，不自动写入，要先确认</span>
          {!thresholdResult.ok && <span className="text-sm text-destructive">{thresholdResult.error}</span>}
        </span>
      </DescRow>
      <DescRow label="最小变动">
        <span className="flex items-center gap-2">
          <PercentInput value={p.minChange} onChange={p.onMinChange} result={minChangeResult} />
          <span className="text-sm text-muted-foreground">变化小于它时，自动写入会跳过（手动预览写入仍会包含）</span>
          {!minChangeResult.ok && <span className="text-sm text-destructive">{minChangeResult.error}</span>}
        </span>
      </DescRow>
      <DescRow label="美元参考汇率（元/$）">
        <span className="flex flex-wrap items-center gap-2">
          <Input
            className="w-24 text-right tabular-nums"
            inputMode="decimal"
            value={p.usdRate}
            aria-invalid={p.usdRateOk ? undefined : true}
            onChange={(e) => p.onUsdRate(e.target.value)}
          />
          <span className="text-sm text-muted-foreground">用于把成本换算成原厂价的百分比；修改后各厂商分组倍率需要重新写入</span>
          {!p.usdRateOk && <span className="text-sm text-destructive">需为大于 0 且不超过 {MAX_USD_CNY_RATE} 的数</span>}
        </span>
      </DescRow>
      <DescRow label="自动写入">
        <Switch checked={p.autoWrite} onCheckedChange={p.onAutoWrite} />
        <span className="ml-2 text-sm text-muted-foreground">开启后，定价任务只自动写入「跟随」模式的厂商；固定模式要手动写入</span>
      </DescRow>
    </>
  );
}
