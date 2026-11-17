import type { StatsMetrics } from '@/api/types';
import { Dash } from '@/components/cells/Dash';
import { MsCell } from '@/components/cells/MsCell';
import { PercentCell } from '@/components/cells/PercentCell';
import { SpeedCell } from '@/components/cells/SpeedCell';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatFailClasses, formatMs, formatPercent, formatSpeed } from '@/lib/format';

export function SuccessRateCell({ metrics }: { metrics: StatsMetrics | undefined }) {
  if (!metrics || metrics.success_rate === null) return <Dash />;
  return (
    <Tooltip>
      <TooltipTrigger asChild><PercentCell value={metrics.success_rate} /></TooltipTrigger>
      <TooltipContent>
        <div>请求数 {metrics.requests}，失败数 {metrics.fail}</div>
        <div>Wilson 下界 {formatPercent(metrics.wilson)}</div>
        <div>错误类别：{formatFailClasses(metrics)}</div>
      </TooltipContent>
    </Tooltip>
  );
}

export function FirstTokenCell({ metrics }: { metrics: StatsMetrics | undefined }) {
  if (!metrics || metrics.frt_p50 === null) return <Dash />;
  return (
    <Tooltip>
      <TooltipTrigger asChild><MsCell value={metrics.frt_p50} /></TooltipTrigger>
      <TooltipContent>p50 {formatMs(metrics.frt_p50)}，p95 {formatMs(metrics.frt_p95)}</TooltipContent>
    </Tooltip>
  );
}

/** 吐字速度（带悬停说明）。 */
export function SpeedStatCell({ metrics }: { metrics: StatsMetrics | undefined }) {
  if (!metrics || metrics.speed_p50 === null) return <Dash />;
  return (
    <Tooltip>
      <TooltipTrigger asChild><SpeedCell value={metrics.speed_p50} estimated={metrics.speed_estimated} /></TooltipTrigger>
      <TooltipContent>
        <div>p50 {formatSpeed(metrics.speed_p50)}，p95 {formatSpeed(metrics.speed_p95)}</div>
        {metrics.speed_estimated && <div>NewAPI 未打补丁 P1，按整秒估算</div>}
      </TooltipContent>
    </Tooltip>
  );
}
