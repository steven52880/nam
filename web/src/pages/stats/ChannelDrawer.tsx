import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { StatsMetrics, StatsRow } from '@/api/types-stats';
import { Button } from '@/components/ui/button';
import { Dialog, DialogDescription, DialogTitle, SheetContent } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import { FrtChart, RequestsChart, SpeedChart } from './TimeCharts';
import { cny, failClassText, formatHour, ms, pct, speed, speedWithMark } from './fmt';
import { toSeriesData } from './seriesData';
import type { TimeRange } from './range';

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded border bg-muted/50 px-2 py-1.5">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="text-sm font-semibold tabular-nums">{value}</div>
    </div>
  );
}

function KeyMetrics({ m }: { m: StatsMetrics }) {
  return (
    <div className="grid grid-cols-3 gap-2">
      <Stat label="请求数" value={String(m.requests)} />
      <Stat label="成功率" value={pct(m.success_rate)} />
      <Stat label="失败数" value={String(m.fail)} />
      <Stat label="首字延迟 p50 / p95（ms）" value={`${ms(m.frt_p50)} / ${ms(m.frt_p95)}`} />
      <Stat label="吐字速度 p50（token/s）" value={speedWithMark(m)} />
      <Stat label="缓存命中率" value={pct(m.cache_hit)} />
      <Stat label="成本（元）" value={cny(m.cost_cny)} />
      <Stat label="收入（元）" value={cny(m.revenue_cny)} />
      <Stat label="毛利率" value={pct(m.margin)} />
    </div>
  );
}

function DrawerBody({ row, range }: { row: StatsRow; range: TimeRange }) {
  const navigate = useNavigate();
  const q = useQuery({
    queryKey: ['stats', 'series', 'channel', row.key, range.from, range.to, 'hour'],
    queryFn: () => api.statsSeries({ from: range.from, to: range.to, dim: 'channel', key: row.key, bucket: 'hour' }),
  });
  const points = q.data?.points ?? [];
  const data = toSeriesData(points, 'hour');

  return (
    <div className="grid min-h-0 flex-1 content-start gap-3 overflow-y-auto p-4">
      <KeyMetrics m={row} />
      {q.isError && <div className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">逐小时数据加载失败：{errorText(q.error)}</div>}
      {q.isLoading && <div className="text-muted-foreground">加载中…</div>}
      {q.data && (
        <>
          <RequestsChart data={data} />
          <div className="grid grid-cols-2 gap-2">
            <FrtChart data={data} />
            <SpeedChart data={data} />
          </div>
          <div className="overflow-x-auto rounded border">
            <table className="w-full border-collapse text-sm">
              <thead>
                <tr className="border-b bg-muted/50 text-left text-muted-foreground">
                  {['时间', '成功', '失败', '失败类别', '首字延迟', '吐字速度', '缓存率', '成本'].map((h) => (
                    <th key={h} className={cn('whitespace-nowrap px-2 py-1', h !== '时间' && h !== '失败类别' && 'text-right')}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {points.map((p) => (
                  <tr key={p.t} className="border-b last:border-0">
                    <td className="whitespace-nowrap px-2 py-1">{formatHour(p.t)}</td>
                    <td className="px-2 py-1 text-right tabular-nums">{p.success}</td>
                    <td className="px-2 py-1 text-right tabular-nums">{p.fail}</td>
                    <td className="px-2 py-1">{failClassText(p)}</td>
                    <td className="px-2 py-1 text-right tabular-nums">{ms(p.frt_p50)}</td>
                    <td className="px-2 py-1 text-right tabular-nums">{speed(p.speed_p50)}{p.speed_p50 !== null && p.speed_estimated && <span className="ml-0.5 text-xs text-muted-foreground">估</span>}</td>
                    <td className="px-2 py-1 text-right tabular-nums">{pct(p.cache_hit)}</td>
                    <td className="px-2 py-1 text-right tabular-nums">{cny(p.cost_cny)}</td>
                  </tr>
                ))}
                {points.length === 0 && (
                  <tr><td colSpan={8} className="px-2 py-4 text-center text-muted-foreground">该时间范围内暂无数据</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </>
      )}
      <div className="flex justify-end border-t pt-3">
        <Button variant="outline" onClick={() => navigate(`/channels?search=${encodeURIComponent(row.name)}`)}>
          在渠道页打开
        </Button>
      </div>
    </div>
  );
}

export function ChannelDrawer({ row, range, onClose }: { row: StatsRow | null; range: TimeRange; onClose: () => void }) {
  return (
    <Dialog open={row !== null} onOpenChange={(v) => { if (!v) onClose(); }}>
      {row && (
        <SheetContent className="max-w-2xl">
          <div className="border-b px-4 py-3">
            <DialogTitle>{row.name}</DialogTitle>
            <DialogDescription>渠道 #{row.key} · 逐小时明细</DialogDescription>
          </div>
          <DrawerBody key={row.key} row={row} range={range} />
        </SheetContent>
      )}
    </Dialog>
  );
}
