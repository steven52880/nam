import { useQuery } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';
import { useMemo, useState } from 'react';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { StatsDim, StatsRow } from '@/api/types-stats';
import { PageHeader } from '@/components/layout/PageHeader';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';
import { ChannelDrawer } from './stats/ChannelDrawer';
import { Leaderboard } from './stats/Leaderboard';
import { ErrorClassChart, PriceQualityChart } from './stats/ScatterCharts';
import { CostRevenueChart, FrtChart, RequestsChart, SpeedChart } from './stats/TimeCharts';
import { PRESET_LABELS, buildRange, toDateInput, type Preset } from './stats/range';
import { toSeriesData } from './stats/seriesData';

const DIMS: Array<[StatsDim, string]> = [
  ['channel', '渠道'],
  ['model', '模型'],
  ['upstream', '上游'],
  ['vendor', '厂商'],
];

const DATE_CLS = 'h-7 rounded border border-input bg-background px-2 text-base focus:outline-none focus:ring-1 focus:ring-ring';

function ErrorLine({ what, error }: { what: string; error: unknown }) {
  return (
    <div className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">
      {what}加载失败：{errorText(error)}
    </div>
  );
}

export default function StatsPage() {
  const [preset, setPreset] = useState<Preset>('7d');
  const [dim, setDim] = useState<StatsDim>('channel');
  const [start, setStart] = useState(() => toDateInput(new Date(Date.now() - 6 * 86400_000)));
  const [end, setEnd] = useState(() => toDateInput(new Date()));
  const [anchor, setAnchor] = useState(() => Date.now());
  const [drawerRow, setDrawerRow] = useState<StatsRow | null>(null);

  const result = useMemo(() => buildRange(preset, start, end, anchor), [preset, start, end, anchor]);
  const range = result.ok ? result.range : null;

  const overview = useQuery({
    queryKey: ['stats', 'overview', range?.from, range?.to, dim],
    queryFn: () => api.statsOverview({ from: range!.from, to: range!.to, dim }),
    enabled: range !== null,
  });
  const series = useQuery({
    queryKey: ['stats', 'series', range?.from, range?.to, range?.bucket],
    queryFn: () => api.statsSeries({ from: range!.from, to: range!.to, bucket: range!.bucket }),
    enabled: range !== null,
  });

  const data = useMemo(
    () => (range && series.data ? toSeriesData(series.data.points, range.bucket) : []),
    [range, series.data],
  );
  const ov = overview.data;
  const speedNotice = useMemo(() => {
    if (!ov) return null;
    if (!ov.patch_p1) return '吐字速度为按整秒估算（NewAPI 未打补丁 P1）';
    const anyEstimated = ov.totals.speed_estimated || data.some((d) => d.speedEst);
    return anyEstimated ? '部分吐字速度为按整秒估算（这些时段的日志没有 use_time_ms）' : null;
  }, [ov, data]);

  const changePreset = (p: Preset) => {
    setPreset(p);
    setAnchor(Date.now());
  };

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-4">
      <PageHeader
        title="统计"
        description="按渠道、模型、上游、厂商查看请求量、成功率、延迟与成本。"
        actions={
          <Button
            size="sm"
            variant="outline"
            onClick={() => { setAnchor(Date.now()); void overview.refetch(); void series.refetch(); }}
          >
            <RefreshCw className={cn('h-3.5 w-3.5', (overview.isFetching || series.isFetching) && 'animate-spin')} />刷新
          </Button>
        }
      />

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <div className="flex items-center gap-1">
          <span className="text-muted-foreground">时间范围</span>
          {PRESET_LABELS.map(([v, l]) => (
            <Button key={v} size="sm" variant={preset === v ? 'default' : 'outline'} onClick={() => changePreset(v)}>{l}</Button>
          ))}
        </div>
        {preset === 'custom' && (
          <div className="flex items-center gap-1">
            <input type="date" className={DATE_CLS} value={start} max={end || undefined} onChange={(e) => setStart(e.target.value)} />
            <span className="text-muted-foreground">至</span>
            <input type="date" className={DATE_CLS} value={end} min={start || undefined} onChange={(e) => setEnd(e.target.value)} />
          </div>
        )}
        <div className="flex items-center gap-1">
          <span className="text-muted-foreground">维度</span>
          <Select value={dim} onValueChange={(v) => setDim(v as StatsDim)}>
            <SelectTrigger className="w-24"><SelectValue /></SelectTrigger>
            <SelectContent>
              {DIMS.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      </div>

      {!result.ok && <div className="rounded bg-warning/15 px-2 py-1 text-sm text-warning">{result.error}</div>}
      {ov?.error_log_hint && (
        <div className="rounded bg-warning/15 px-2 py-1 text-sm text-warning">
          可能没开错误日志：最近 7 天没有任何错误日志，NewAPI 可能没开 ERROR_LOG_ENABLED，成功率会偏高。
        </div>
      )}
      {speedNotice && <div className="text-sm text-muted-foreground">{speedNotice}</div>}
      {overview.isError && <ErrorLine what="概览" error={overview.error} />}
      {series.isError && <ErrorLine what="时间序列" error={series.error} />}
      {(overview.isLoading || series.isLoading) && range && <div className="text-muted-foreground">加载中…</div>}

      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        <RequestsChart data={data} />
        <CostRevenueChart data={data} />
        <div className="grid grid-cols-2 gap-3 xl:col-span-2">
          <FrtChart data={data} />
          <SpeedChart data={data} />
        </div>
        <PriceQualityChart points={ov?.scatter ?? []} />
        <ErrorClassChart classes={ov?.error_classes ?? {}} />
      </div>

      <Leaderboard rows={ov?.rows ?? []} dim={dim} onRowClick={dim === 'channel' ? setDrawerRow : undefined} />

      {range && <ChannelDrawer row={drawerRow} range={range} onClose={() => setDrawerRow(null)} />}
    </div>
  );
}
