import { FAIL_CLASS_LABELS, type FailClass, type StatsMetrics } from '@/api/types';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';

export function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

/** 0–1 的比率 → 保留 1 位小数的百分比；空值「—」。 */
export function formatRate(fraction: number | null | undefined): string {
  if (fraction === null || fraction === undefined) return '—';
  return `${(fraction * 100).toFixed(1)}%`;
}

export function formatMs(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—';
  return `${Math.round(value)} ms`;
}

export function formatSpeed(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—';
  return `${value.toFixed(1)} t/s`;
}

function failClassText(metrics: StatsMetrics): string {
  const parts = (Object.keys(FAIL_CLASS_LABELS) as FailClass[])
    .filter((key) => (metrics.fail_by_class[key] ?? 0) > 0)
    .map((key) => `${FAIL_CLASS_LABELS[key]} ${metrics.fail_by_class[key]}`);
  return parts.length > 0 ? parts.join('、') : '无';
}

export function SuccessRateCell({ metrics }: { metrics: StatsMetrics | undefined }) {
  if (!metrics || metrics.success_rate === null) return <Dash />;
  return (
    <Tooltip>
      <TooltipTrigger asChild><span className="tabular-nums">{formatRate(metrics.success_rate)}</span></TooltipTrigger>
      <TooltipContent>
        <div>请求数 {metrics.requests}，失败数 {metrics.fail}</div>
        <div>Wilson 下界 {formatRate(metrics.wilson)}</div>
        <div>错误类别：{failClassText(metrics)}</div>
      </TooltipContent>
    </Tooltip>
  );
}

export function FirstTokenCell({ metrics }: { metrics: StatsMetrics | undefined }) {
  if (!metrics || metrics.frt_p50 === null) return <Dash />;
  return (
    <Tooltip>
      <TooltipTrigger asChild><span className="tabular-nums">{formatMs(metrics.frt_p50)}</span></TooltipTrigger>
      <TooltipContent>p50 {formatMs(metrics.frt_p50)}，p95 {formatMs(metrics.frt_p95)}</TooltipContent>
    </Tooltip>
  );
}

export function SpeedCell({ metrics }: { metrics: StatsMetrics | undefined }) {
  if (!metrics || metrics.speed_p50 === null) return <Dash />;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="tabular-nums">
          {formatSpeed(metrics.speed_p50)}
          {metrics.speed_estimated && <span className="ml-0.5 text-xs text-muted-foreground">估</span>}
        </span>
      </TooltipTrigger>
      <TooltipContent>
        <div>p50 {formatSpeed(metrics.speed_p50)}，p95 {formatSpeed(metrics.speed_p95)}</div>
        {metrics.speed_estimated && <div>NewAPI 未打补丁 P1，按整秒估算</div>}
      </TooltipContent>
    </Tooltip>
  );
}
