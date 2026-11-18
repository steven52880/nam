import { useCallback, useMemo, useRef, type ReactNode } from 'react';
import { Download } from 'lucide-react';
import type { StatsDim, StatsRow } from '@/api/types-stats';
import { DataTable, numberCol, textCol, type DataTableHandle, type SortSpec, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import {
  DASH,
  formatDate,
  formatFailClasses,
  formatInt,
  formatMs,
  formatPercent,
  formatSignificant,
  formatSpeed,
} from '@/lib/format';

interface Metric {
  id: string;
  label: string;
  /** 默认列宽，rem。 */
  width: number;
  get: (r: StatsRow) => number | null;
  show: (r: StatsRow) => string;
  csv?: (r: StatsRow) => string;
}

const METRICS: Metric[] = [
  { id: 'requests', label: '请求数', width: 6, get: (r) => r.requests, show: (r) => formatInt(r.requests) },
  { id: 'rate', label: '成功率', width: 6, get: (r) => r.success_rate, show: (r) => formatPercent(r.success_rate) },
  { id: 'fail', label: '失败数', width: 6, get: (r) => r.fail, show: (r) => formatInt(r.fail) },
  { id: 'frt50', label: '首字延迟 p50（ms）', width: 10.5, get: (r) => r.frt_p50, show: (r) => formatMs(r.frt_p50, { unit: false }) },
  { id: 'frt95', label: '首字延迟 p95（ms）', width: 10.5, get: (r) => r.frt_p95, show: (r) => formatMs(r.frt_p95, { unit: false }) },
  {
    id: 'speed',
    label: '吐字速度（token/s）',
    width: 12,
    get: (r) => r.speed_p50,
    show: (r) => formatSpeed(r.speed_p50, { unit: false, estimated: r.speed_estimated }),
    csv: (r) => formatSpeed(r.speed_p50, { unit: false }),
  },
  { id: 'cost', label: '成本（元）', width: 7.5, get: (r) => r.cost_cny, show: (r) => formatSignificant(r.cost_cny) },
  { id: 'revenue', label: '收入（元）', width: 7.5, get: (r) => r.revenue_cny, show: (r) => formatSignificant(r.revenue_cny) },
  { id: 'margin', label: '毛利率', width: 6, get: (r) => r.margin, show: (r) => formatPercent(r.margin) },
];

const DIM_LABELS: Record<StatsDim, string> = { channel: '渠道', model: '模型', upstream: '上游', vendor: '厂商' };

/** 原来的默认排序：请求数从多到少。 */
const DEFAULT_SORT: SortSpec[] = [{ id: 'requests', desc: true }];

function csvCell(s: string): string {
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

function exportCsv(rows: StatsRow[], dim: StatsDim) {
  const head = [DIM_LABELS[dim], ...METRICS.map((c) => c.label)];
  const lines = rows.map((r) => [r.name, ...METRICS.map((c) => (c.csv ?? c.show)(r))]);
  const text = [head, ...lines].map((l) => l.map(csvCell).join(',')).join('\r\n');
  const url = URL.createObjectURL(new Blob(['\uFEFF', text], { type: 'text/csv;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = `nam-stats-${dim}-${formatDate(new Date())}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

/** 原来整行带 title 提示失败类别；表格没有行级 title，改挂在每个单元格的内容上。 */
function withFailTitle(r: StatsRow, node: ReactNode): ReactNode {
  const classes = formatFailClasses(r, { style: 'compact' });
  return <span title={classes === DASH ? undefined : `失败类别：${classes}`}>{node}</span>;
}

const getRowId = (r: StatsRow) => r.key;

export function Leaderboard({
  rows,
  dim,
  onRowClick,
}: {
  rows: StatsRow[];
  dim: StatsDim;
  onRowClick?: (r: StatsRow) => void;
}) {
  const tableRef = useRef<DataTableHandle<StatsRow>>(null);

  const columns = useMemo<TableColumn<StatsRow>[]>(
    () => [
      textCol<StatsRow>('name', DIM_LABELS[dim], (r) => r.name, {
        width: 16,
        cell: (r) => withFailTitle(r, r.name),
      }),
      ...METRICS.map((c) =>
        numberCol<StatsRow>(c.id, c.label, c.get, {
          width: c.width,
          cell: (r) => withFailTitle(r, c.show(r)),
        }),
      ),
    ],
    [dim],
  );

  // 导出顺序与表格当前排序一致。
  const onExport = useCallback(() => {
    exportCsv(tableRef.current?.getRows() ?? rows, dim);
  }, [rows, dim]);

  return (
    <DataTable<StatsRow>
      layoutId="stats.leaderboard"
      tableRef={tableRef}
      data={rows}
      columns={columns}
      getRowId={getRowId}
      defaultSort={DEFAULT_SORT}
      onRowClick={onRowClick}
      toolbar={
        <>
          <span className="text-sm font-medium">排行榜（按{DIM_LABELS[dim]}）</span>
          <Button size="sm" variant="outline" disabled={rows.length === 0} onClick={onExport}>
            <Download className="h-3.5 w-3.5" />导出 CSV
          </Button>
        </>
      }
    />
  );
}
