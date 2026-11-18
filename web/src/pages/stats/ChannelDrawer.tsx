import { useNavigate } from 'react-router-dom';
import { errorText } from '@/api/errors';
import { useChannelSeries } from '@/api/queries';
import type { SeriesPoint, StatsMetrics, StatsRow } from '@/api/types-stats';
import { DataTable, numberCol, textCol, type TableColumn } from '@/components/table';
import { Banner } from '@/components/ui/banner';
import { Button } from '@/components/ui/button';
import { DrawerShell } from '@/components/ui/dialog';
import { FrtChart, RequestsChart, SpeedChart } from './TimeCharts';
import { SpeedCell } from '@/components/cells/SpeedCell';
import { formatFailClasses, formatHour, formatMs, formatPercent, formatSignificant, formatSpeed } from '@/lib/format';
import { toSeriesData } from './seriesData';
import type { TimeRange } from './range';

const HOURLY_COLUMNS: TableColumn<SeriesPoint>[] = [
  numberCol<SeriesPoint>('time', '时间', (p) => p.t, {
    width: 8.5,
    align: 'left',
    cell: (p) => formatHour(p.t),
  }),
  numberCol<SeriesPoint>('success', '成功', (p) => p.success, { width: 4.5 }),
  numberCol<SeriesPoint>('fail', '失败', (p) => p.fail, { width: 4.5 }),
  textCol<SeriesPoint>('fail_classes', '失败类别', (p) => formatFailClasses(p, { style: 'compact' }), { width: 9 }),
  numberCol<SeriesPoint>('frt_p50', '首字延迟', (p) => p.frt_p50, {
    width: 6,
    cell: (p) => formatMs(p.frt_p50, { unit: false }),
  }),
  numberCol<SeriesPoint>('speed_p50', '吐字速度', (p) => p.speed_p50, {
    width: 6.5,
    cell: (p) =>
      p.speed_p50 === null ? formatSpeed(null) : <SpeedCell value={p.speed_p50} estimated={p.speed_estimated} unit={false} />,
  }),
  numberCol<SeriesPoint>('cache_hit', '缓存率', (p) => p.cache_hit, {
    width: 5.5,
    cell: (p) => formatPercent(p.cache_hit),
  }),
  numberCol<SeriesPoint>('cost_cny', '成本', (p) => p.cost_cny, {
    width: 6,
    cell: (p) => formatSignificant(p.cost_cny),
  }),
];

const HOURLY_STATUS = { empty: '该时间范围内暂无数据' };
const hourlyRowId = (p: SeriesPoint) => String(p.t);

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
      <Stat label="成功率" value={formatPercent(m.success_rate)} />
      <Stat label="失败数" value={String(m.fail)} />
      <Stat label="首字延迟 p50 / p95（ms）" value={`${formatMs(m.frt_p50, { unit: false })} / ${formatMs(m.frt_p95, { unit: false })}`} />
      <Stat label="吐字速度 p50（token/s）" value={formatSpeed(m.speed_p50, { unit: false, estimated: m.speed_estimated })} />
      <Stat label="缓存命中率" value={formatPercent(m.cache_hit)} />
      <Stat label="成本（元）" value={formatSignificant(m.cost_cny)} />
      <Stat label="收入（元）" value={formatSignificant(m.revenue_cny)} />
      <Stat label="毛利率" value={formatPercent(m.margin)} />
    </div>
  );
}

function DrawerBody({ row, range }: { row: StatsRow; range: TimeRange }) {
  const navigate = useNavigate();
  const q = useChannelSeries(row.key, range);
  const points = q.data?.points ?? [];
  const data = toSeriesData(points, 'hour');

  return (
    <div className="grid content-start gap-3 p-4">
      <KeyMetrics m={row} />
      {q.isError && <Banner tone="error">逐小时数据加载失败：{errorText(q.error)}</Banner>}
      {q.isLoading && <div className="text-muted-foreground">加载中…</div>}
      {q.data && (
        <>
          <RequestsChart data={data} />
          <div className="grid grid-cols-2 gap-2">
            <FrtChart data={data} />
            <SpeedChart data={data} />
          </div>
          <DataTable<SeriesPoint>
            layoutId="stats.hourly"
            className="max-h-80"
            data={points}
            columns={HOURLY_COLUMNS}
            getRowId={hourlyRowId}
            status={HOURLY_STATUS}
          />
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
    <DrawerShell
      open={row !== null}
      onOpenChange={(v) => { if (!v) onClose(); }}
      size="2xl"
      title={row?.name}
      description={row ? `渠道 #${row.key} · 逐小时明细` : undefined}
    >
      {row && <DrawerBody key={row.key} row={row} range={range} />}
    </DrawerShell>
  );
}
