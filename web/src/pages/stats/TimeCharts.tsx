import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ComposedChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { isSpeedEstimated, type SeriesDatum } from './seriesData';
import { AXIS_TICK, BAR_CURSOR, CHART_MARGIN, COLORS, ChartCard, GRID_STROKE, TOOLTIP_PROPS } from './chartTheme';
import { formatMs, formatSignificant, formatSpeed } from '@/lib/format';

function num(v: unknown): number | null {
  return typeof v === 'number' ? v : null;
}

function Grid() {
  return <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" vertical={false} />;
}

export function RequestsChart({ data }: { data: SeriesDatum[] }) {
  return (
    <ChartCard title="请求量与成功率" empty={data.length === 0}>
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart data={data} margin={{ ...CHART_MARGIN, right: -8 }}>
          <Grid />
          <XAxis dataKey="label" tick={AXIS_TICK} minTickGap={24} />
          <YAxis yAxisId="l" tick={AXIS_TICK} allowDecimals={false} />
          <YAxis yAxisId="r" orientation="right" domain={[0, 100]} tick={AXIS_TICK} unit="%" />
          <Tooltip
            {...TOOLTIP_PROPS}
            cursor={BAR_CURSOR}
            formatter={(v, name) => (name === '成功率' ? `${(num(v) ?? 0).toFixed(1)}%` : String(v))}
          />
          <Legend wrapperStyle={{ fontSize: '0.6875rem' }} />
          <Bar yAxisId="l" dataKey="requests" name="请求数" fill={COLORS.primary} fillOpacity={0.75} />
          <Line yAxisId="r" dataKey="rate" name="成功率" stroke={COLORS.success} dot={false} strokeWidth={2} connectNulls />
        </ComposedChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

function PercentileChart({
  title,
  data,
  k50,
  k95,
  fmt,
  unit,
  markEstimated = false,
}: {
  title: string;
  data: SeriesDatum[];
  k50: 'frt50' | 'sp50';
  k95: 'frt95' | 'sp95';
  fmt: (v: number | null) => string;
  unit: string;
  /** 为 true 时，估算点的 tooltip 追加「（估算）」。 */
  markEstimated?: boolean;
}) {
  const empty = data.every((d) => d[k50] === null && d[k95] === null);
  return (
    <ChartCard title={title} empty={empty}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={CHART_MARGIN}>
          <Grid />
          <XAxis dataKey="label" tick={AXIS_TICK} minTickGap={24} />
          <YAxis tick={AXIS_TICK} />
          <Tooltip
            {...TOOLTIP_PROPS}
            formatter={(v, _name, item) =>
              `${fmt(num(v))} ${unit}${markEstimated && isSpeedEstimated(item.payload) ? '（估算）' : ''}`
            }
          />
          <Legend wrapperStyle={{ fontSize: '0.6875rem' }} />
          <Line dataKey={k50} name="p50" stroke={COLORS.primary} dot={false} strokeWidth={2} connectNulls />
          <Line dataKey={k95} name="p95" stroke={COLORS.warning} dot={false} strokeWidth={2} connectNulls />
        </LineChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

export function FrtChart({ data }: { data: SeriesDatum[] }) {
  return <PercentileChart title="首字延迟 p50 / p95" data={data} k50="frt50" k95="frt95" fmt={(v) => formatMs(v, { unit: false })} unit="ms" />;
}

export function SpeedChart({ data }: { data: SeriesDatum[] }) {
  return <PercentileChart title="吐字速度 p50 / p95" data={data} k50="sp50" k95="sp95" fmt={(v) => formatSpeed(v, { unit: false })} unit="token/s" markEstimated />;
}

export function CostRevenueChart({ data }: { data: SeriesDatum[] }) {
  return (
    <ChartCard title="成本 vs 收入（元）" empty={data.length === 0}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} margin={CHART_MARGIN}>
          <Grid />
          <XAxis dataKey="label" tick={AXIS_TICK} minTickGap={24} />
          <YAxis tick={AXIS_TICK} />
          <Tooltip {...TOOLTIP_PROPS} cursor={BAR_CURSOR} formatter={(v) => formatSignificant(num(v))} />
          <Legend wrapperStyle={{ fontSize: '0.6875rem' }} />
          <Bar dataKey="cost" name="成本" fill={COLORS.danger} fillOpacity={0.75} />
          <Bar dataKey="revenue" name="收入" fill={COLORS.success} fillOpacity={0.75} />
        </BarChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}
