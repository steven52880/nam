import { useMemo, useState } from 'react';
import { Download } from 'lucide-react';
import type { StatsDim, StatsRow } from '@/api/types-stats';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { cny, failClassText, int, ms, pct, speed } from './fmt';

type Getter = (r: StatsRow) => number | string | null;

interface Col {
  id: string;
  label: string;
  num: boolean;
  get: Getter;
  show: (r: StatsRow) => string;
  csv?: (r: StatsRow) => string;
}

const COLS: Col[] = [
  { id: 'requests', label: '请求数', num: true, get: (r) => r.requests, show: (r) => int(r.requests) },
  { id: 'rate', label: '成功率', num: true, get: (r) => r.success_rate, show: (r) => pct(r.success_rate) },
  { id: 'fail', label: '失败数', num: true, get: (r) => r.fail, show: (r) => int(r.fail) },
  { id: 'frt50', label: '首字延迟 p50（ms）', num: true, get: (r) => r.frt_p50, show: (r) => ms(r.frt_p50) },
  { id: 'frt95', label: '首字延迟 p95（ms）', num: true, get: (r) => r.frt_p95, show: (r) => ms(r.frt_p95) },
  {
    id: 'speed',
    label: '吐字速度（token/s）',
    num: true,
    get: (r) => r.speed_p50,
    show: (r) => (r.speed_p50 === null ? '—' : `${speed(r.speed_p50)}${r.speed_estimated ? ' 估' : ''}`),
    csv: (r) => speed(r.speed_p50),
  },
  { id: 'cost', label: '成本（元）', num: true, get: (r) => r.cost_cny, show: (r) => cny(r.cost_cny) },
  { id: 'revenue', label: '收入（元）', num: true, get: (r) => r.revenue_cny, show: (r) => cny(r.revenue_cny) },
  { id: 'margin', label: '毛利率', num: true, get: (r) => r.margin, show: (r) => pct(r.margin) },
];

const DIM_LABELS: Record<StatsDim, string> = { channel: '渠道', model: '模型', upstream: '上游', vendor: '厂商' };

function csvCell(s: string): string {
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

function ymd(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}`;
}

function exportCsv(rows: StatsRow[], dim: StatsDim) {
  const head = [DIM_LABELS[dim], ...COLS.map((c) => c.label)];
  const lines = rows.map((r) => [r.name, ...COLS.map((c) => (c.csv ?? c.show)(r))]);
  const text = [head, ...lines].map((l) => l.map(csvCell).join(',')).join('\r\n');
  const url = URL.createObjectURL(new Blob(['\uFEFF', text], { type: 'text/csv;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = `nam-stats-${dim}-${ymd(new Date())}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

function compare(a: number | string | null, b: number | string | null): number {
  if (a === null && b === null) return 0;
  if (a === null) return 1;
  if (b === null) return -1;
  if (typeof a === 'number' && typeof b === 'number') return a - b;
  return String(a).localeCompare(String(b), 'zh');
}

export function Leaderboard({
  rows,
  dim,
  onRowClick,
}: {
  rows: StatsRow[];
  dim: StatsDim;
  onRowClick?: (r: StatsRow) => void;
}) {
  const [sort, setSort] = useState<{ id: string; desc: boolean }>({ id: 'requests', desc: true });

  const sorted = useMemo(() => {
    const get: Getter = sort.id === 'name' ? (r) => r.name : (COLS.find((c) => c.id === sort.id)?.get ?? ((r) => r.requests));
    const out = [...rows].sort((x, y) => {
      const a = get(x);
      const b = get(y);
      // 空值永远排最后，不随升降序翻转。
      if (a === null || b === null) return compare(a, b);
      return sort.desc ? -compare(a, b) : compare(a, b);
    });
    return out;
  }, [rows, sort]);

  const toggle = (id: string) => setSort((s) => (s.id === id ? { id, desc: !s.desc } : { id, desc: true }));
  const mark = (id: string) => (sort.id === id ? (sort.desc ? ' ↓' : ' ↑') : '');

  return (
    <div className="rounded border bg-card">
      <div className="flex items-center justify-between border-b px-3 py-1.5">
        <span className="text-sm font-medium">排行榜（按{DIM_LABELS[dim]}）</span>
        <Button size="sm" variant="outline" disabled={rows.length === 0} onClick={() => exportCsv(sorted, dim)}>
          <Download className="h-3.5 w-3.5" />导出 CSV
        </Button>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b bg-muted/50 text-left text-muted-foreground">
              <th className="cursor-pointer select-none whitespace-nowrap px-2 py-1" onClick={() => toggle('name')}>
                {DIM_LABELS[dim]}{mark('name')}
              </th>
              {COLS.map((c) => (
                <th key={c.id} className="cursor-pointer select-none whitespace-nowrap px-2 py-1 text-right" onClick={() => toggle(c.id)}>
                  {c.label}{mark(c.id)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.map((r) => (
              <tr
                key={r.key}
                title={failClassText(r) === '—' ? undefined : `失败类别：${failClassText(r)}`}
                className={cn('border-b last:border-0 hover:bg-accent/50', onRowClick && 'cursor-pointer')}
                onClick={onRowClick ? () => onRowClick(r) : undefined}
              >
                <td className="max-w-[260px] truncate px-2 py-1">{r.name}</td>
                {COLS.map((c) => (
                  <td key={c.id} className="whitespace-nowrap px-2 py-1 text-right tabular-nums">{c.show(r)}</td>
                ))}
              </tr>
            ))}
            {sorted.length === 0 && (
              <tr><td colSpan={COLS.length + 1} className="px-2 py-6 text-center text-muted-foreground">暂无数据</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
