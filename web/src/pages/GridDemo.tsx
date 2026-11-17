import { useCallback, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { PageHeader } from '@/components/layout/PageHeader';
import { DataTable, numberCol, textCol, type DataTableHandle, type GridRange, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { MenuItem } from '@/components/ui/menu-items';

interface DemoRow {
  id: string;
  name: string;
  upstream: string;
  discount: string;
  priority: string;
  weight: string;
  cost: number;
  note: string;
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
    note: i % 4 === 0 ? '' : `备注 ${i + 1}`,
  }));
}

const POSITIVE = (v: string): string | null => (Number(v) > 0 ? null : '上游分组折扣必须大于 0');
const INTEGER = (label: string, min: number) => (v: string): string | null =>
  /^-?\d+$/.test(v) && Number(v) >= min ? null : `${label}必须是不小于 ${min} 的整数`;

/** 演示用的"新增列"：插在默认顺序里"权重"后面，用来演示已调整过的布局遇到新列时的对账。 */
const EXTRA_COLUMN_ID = 'note';

export default function GridDemoPage() {
  const [rows, setRows] = useState<DemoRow[]>(makeRows);
  const [dirty, setDirty] = useState<Set<string>>(new Set());
  const [withExtra, setWithExtra] = useState(false);
  const [search, setSearch] = useState('');
  const tableRef = useRef<DataTableHandle<DemoRow>>(null);

  const columns = useMemo<TableColumn<DemoRow>[]>(() => {
    const cols: TableColumn<DemoRow>[] = [
      textCol<DemoRow>('name', '名称', (r) => r.name, {
        width: 12.5,
        editable: true,
        validate: (v, r) => (v.trim() === '' ? '名称不能为空' : rows.some((x) => x.id !== r.id && x.name === v) ? '名称不能重复' : null),
      }),
      textCol<DemoRow>('upstream', '上游', (r) => r.upstream, {
        width: 6.25,
        readOnlyReason: '上游由令牌决定，不能在这里修改',
      }),
      numberCol<DemoRow>('discount', '上游分组折扣', (r) => Number(r.discount), {
        width: 7,
        editable: true,
        toText: (r) => r.discount,
        validate: POSITIVE,
      }),
      numberCol<DemoRow>('priority', '优先级', (r) => Number(r.priority), {
        width: 5,
        editable: true,
        toText: (r) => r.priority,
        validate: INTEGER('优先级', -9999),
      }),
      numberCol<DemoRow>('weight', '权重', (r) => Number(r.weight), {
        width: 5,
        editable: true,
        toText: (r) => r.weight,
        validate: INTEGER('权重', 0),
      }),
      numberCol<DemoRow>('cost', '成本（元）', (r) => r.cost, {
        width: 6.25,
        readOnlyReason: '成本由系统计算得出，不能手动修改',
      }),
    ];
    if (withExtra) {
      // 默认顺序里放在"权重"后面；已保存过顺序的布局里，它会出现在"权重"附近，其它列不动
      const at = cols.findIndex((c) => c.id === 'weight') + 1;
      cols.splice(at, 0, textCol<DemoRow>(EXTRA_COLUMN_ID, '备注（新增）', (r) => r.note, { width: 8, editable: true }));
    }
    return cols;
  }, [rows, withExtra]);

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

  const rowMenu = (row: DemoRow): MenuItem[] => [
    { key: 'info', label: '显示行信息', onSelect: () => toast.info(`${row.name}（${row.upstream}）`) },
    {
      key: 'reset-cost',
      label: '重置成本',
      disabled: true,
      title: '演示：禁用项悬停时显示原因',
    },
    {
      key: 'remove',
      label: '删除此行',
      destructive: true,
      separatorBefore: true,
      onSelect: () => setRows((prev) => prev.filter((r) => r.id !== row.id)),
    },
  ];

  const keyword = search.trim().toLowerCase();
  const data = useMemo(
    () => (keyword ? rows.filter((r) => r.name.toLowerCase().includes(keyword)) : rows),
    [rows, keyword],
  );

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      <PageHeader
        title="表格组件演示"
        description="开发用页面：框选编辑、复制粘贴、分组折叠、排序、列宽与自动调整、拖列、显示隐藏列、行右键菜单。"
        actions={
          <>
            <Button variant="outline" onClick={() => tableRef.current?.autoFitAll()}>自动调整全部列</Button>
            <Button variant="outline" onClick={() => tableRef.current?.resetLayout()}>恢复默认布局</Button>
            <Button variant={withExtra ? 'default' : 'outline'} onClick={() => setWithExtra((v) => !v)}>
              {withExtra ? '移除新增列' : '新增一列'}
            </Button>
          </>
        }
      />
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
        <span>先调整列宽、顺序，再点「新增一列」：新列出现在「权重」后面，其它列的宽度和顺序不变。</span>
        <label className="flex items-center gap-1">
          搜索（命中时分组全部展开）
          <Input className="w-40 text-foreground" value={search} onChange={(e) => setSearch(e.target.value)} />
        </label>
      </div>
      <DataTable<DemoRow>
        layoutId="demo.grid"
        className="min-h-0 flex-1"
        tableRef={tableRef}
        data={data}
        columns={columns}
        getRowId={(r) => r.id}
        selection="cells"
        groupBy={(r) => r.upstream}
        groupLabel={(key) => key}
        groupSummary={(_, rs) => `成本合计 ${rs.reduce((s, r) => s + r.cost, 0).toFixed(2)} 元`}
        expandAll={keyword !== ''}
        isDirty={(rowId, colId) => dirty.has(`${rowId}\u0000${colId}`)}
        onEdit={apply}
        onCopy={onCopy}
        onPaste={onPaste}
        rowMenu={rowMenu}
        rowMenuHeader={(r) => r.name}
        toolbar={<span className="text-sm text-muted-foreground">共 {rows.length} 行，已改动 {dirty.size} 格</span>}
      />
    </div>
  );
}
