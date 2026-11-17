import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Scatter, ScatterChart, Tooltip, XAxis, YAxis, ZAxis } from 'recharts';
import { FAIL_CLASS_LABELS, type FailClass, type ScatterPoint, type StatsOverview } from '@/api/types-stats';
import { AXIS_TICK, BAR_CURSOR, COLORS, ChartCard, GRID_STROKE, TOOLTIP_PROPS } from './chartTheme';
import { CLASS_ORDER, cny, pct } from './fmt';

interface ScatterDatum {
  name: string;
  vendor: string;
  x: number;
  y: number;
  z: number;
}

function toDatum(p: ScatterPoint): ScatterDatum | null {
  if (p.std_cost_price === null || p.wilson === null) return null;
  return { name: p.name, vendor: p.vendor, x: p.std_cost_price, y: p.wilson * 100, z: Math.max(1, p.requests) };
}

function ScatterTip({ active, payload }: { active?: boolean; payload?: Array<{ payload?: ScatterDatum }> }) {
  const d = payload?.[0]?.payload;
  if (!active || !d) return null;
  return (
    <div style={TOOLTIP_PROPS.contentStyle}>
      <div className="font-medium">{d.name}</div>
      <div className="text-muted-foreground">厂商：{d.vendor || '未归属'}</div>
      <div>标准负载成本价：{cny(d.x)}</div>
      <div>成功率下界：{pct(d.y / 100)}</div>
      <div>请求数：{d.z}</div>
    </div>
  );
}

export function PriceQualityChart({ points }: { points: StatsOverview['scatter'] }) {
  const data = points.map(toDatum).filter((d): d is ScatterDatum => d !== null);
  return (
    <ChartCard title="价格-质量（横：标准负载成本价，纵：成功率下界 %）" empty={data.length === 0}>
      <ResponsiveContainer width="100%" height="100%">
        <ScatterChart margin={{ top: 6, right: 12, bottom: 0, left: -8 }}>
          <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" />
          <XAxis type="number" dataKey="x" name="成本价" tick={AXIS_TICK} domain={['auto', 'auto']} />
          <YAxis type="number" dataKey="y" name="成功率下界" tick={AXIS_TICK} domain={[0, 100]} unit="%" />
          <ZAxis type="number" dataKey="z" range={[40, 360]} />
          <Tooltip content={<ScatterTip />} cursor={{ strokeDasharray: '3 3' }} />
          <Scatter data={data} fill={COLORS.primary} fillOpacity={0.6} />
        </ScatterChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

interface ClassDatum {
  cls: FailClass;
  label: string;
  count: number;
}

export function ErrorClassChart({ classes }: { classes: StatsOverview['error_classes'] }) {
  const data: ClassDatum[] = CLASS_ORDER.map((cls) => ({ cls, label: FAIL_CLASS_LABELS[cls], count: classes[cls] ?? 0 })).filter(
    (d) => d.count > 0,
  );
  return (
    <ChartCard title="错误类别分布" empty={data.length === 0}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} layout="vertical" margin={{ top: 6, right: 16, bottom: 0, left: 8 }}>
          <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" horizontal={false} />
          <XAxis type="number" tick={AXIS_TICK} allowDecimals={false} />
          <YAxis type="category" dataKey="label" tick={AXIS_TICK} width={72} />
          <Tooltip {...TOOLTIP_PROPS} cursor={BAR_CURSOR} formatter={(v) => String(v)} />
          <Bar dataKey="count" name="次数">
            {data.map((d) => (
              <Cell key={d.cls} fill={COLORS.danger} fillOpacity={0.75} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}
