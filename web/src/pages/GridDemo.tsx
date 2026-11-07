import { useCallback, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { DataGrid, type GridColumn, type GridRange } from '@/components/data-grid';
import { PageHeader } from '@/components/layout/PageHeader';

interface DemoRow {
  id: string;
  name: string;
  upstream: string;
  discount: string;
  priority: string;
  weight: string;
  cost: number;
}

const UPSTREAMS = ['上游甲', '上游乙', '上游丙'];

function makeRows(): DemoRow[] {
  return Array.from({ length: 30 }, (_, i) => ({
    id: `r${i + 1}`,
    name: `nam-${UPSTREAMS[i % 3]}-令牌${i + 1}`,
    upstream: UPSTREAMS[i % 3],
    discount: (0.5 + (i % 7) * 0.1).toFixed(2),
    priority: String(i % 4),
    weight: String(10 + (i % 5)),
    cost: Math.round((1.2 + (i % 9) * 0.35) * 100) / 100,
  }));
}

const POSITIVE = (v: string): string | null => (Number(v) > 0 ? null : '上游分组折扣必须大于 0');
const INTEGER = (label: string, min: number) => (v: string): string | null =>
  /^-?\d+$/.test(v) && Number(v) >= min ? null : `${label}必须是不小于 ${min} 的整数`;

export default function GridDemoPage() {
  const [rows, setRows] = useState<DemoRow[]>(makeRows);
  const [dirty, setDirty] = useState<Set<string>>(new Set());

  const columns = useMemo<GridColumn<DemoRow>[]>(
    () => [
      { id: 'name', header: '名称', width: 200, accessor: (r) => r.name, editable: true,
        validate: (v, r) => (v.trim() === '' ? '名称不能为空' : rows.some((x) => x.id !== r.id && x.name === v) ? '名称不能重复' : null) },
      { id: 'upstream', header: '上游', width: 100, accessor: (r) => r.upstream, readOnlyReason: '上游由令牌决定，不能在这里修改' },
      { id: 'discount', header: '上游分组折扣', width: 110, align: 'right', accessor: (r) => Number(r.discount), toText: (r) => r.discount, editable: true, validate: POSITIVE },
      { id: 'priority', header: '优先级', width: 80, align: 'right', accessor: (r) => Number(r.priority), toText: (r) => r.priority, editable: true, validate: INTEGER('优先级', -9999) },
      { id: 'weight', header: '权重', width: 80, align: 'right', accessor: (r) => Number(r.weight), toText: (r) => r.weight, editable: true, validate: INTEGER('权重', 0) },
      { id: 'cost', header: '成本（元）', width: 100, align: 'right', accessor: (r) => r.cost, readOnlyReason: '成本由系统计算得出，不能手动修改' },
    ],
    [rows],
  );

  const apply = useCallback((rowId: string, colId: string, value: string) => {
    setRows((prev) => prev.map((r) => (r.id === rowId ? { ...r, [colId]: value } : r)));
    setDirty((prev) => new Set(prev).add(`${rowId}\u0000${colId}`));
  }, []);

  const onCopy = (range: GridRange): string | void => {
    if (range.rowIds.length !== 1) {
      toast.warning('只能复制单行');
      return;
    }
    const row = rows.find((r) => r.id === range.rowIds[0]);
    if (!row) return;
    const text = range.columnIds.map((c) => String(row[c as keyof DemoRow] ?? '')).join('\t');
    toast.success(`已复制 ${range.columnIds.length} 列`);
    return text;
  };

  const onPaste = (range: GridRange, text: string) => {
    const parts = text.split('\t');
    let changed = 0;
    for (const rowId of range.rowIds) {
      range.columnIds.forEach((colId, i) => {
        const col = columns.find((c) => c.id === colId);
        const value = parts[i % parts.length];
        const row = rows.find((r) => r.id === rowId);
        if (!col?.editable || !row || value === undefined) return;
        if (col.validate?.(value, row)) return;
        apply(rowId, colId, value);
        changed += 1;
      });
    }
    toast.info(`已粘贴，修改了 ${changed} 格`);
  };

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      <PageHeader title="表格组件演示" description="开发用页面：选区、编辑、复制粘贴、分组、列显示。" />
      <DataGrid
        className="min-h-0 flex-1"
        data={rows}
        columns={columns}
        getRowId={(r) => r.id}
        groupBy={(r) => r.upstream}
        groupSummary={(_, rs) => `成本合计 ${rs.reduce((s, r) => s + r.cost, 0).toFixed(2)} 元`}
        isDirty={(rowId, colId) => dirty.has(`${rowId}\u0000${colId}`)}
        onEdit={apply}
        onCopy={onCopy}
        onPaste={onPaste}
        toolbar={<span className="text-sm text-muted-foreground">共 {rows.length} 行，已改动 {dirty.size} 格</span>}
      />
    </div>
  );
}
