import { useMemo, useState } from 'react';
import { useAudit } from '@/api/queries';
import type { AuditLog } from '@/api/types';
import { PageHeader } from '@/components/layout/PageHeader';
import { DataTable, numberCol, textCol, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import { FilterBar } from '@/components/filters/FilterBar';
import { ALL_FILTER, FilterSelect } from '@/components/filters/FilterSelect';
import { SearchInput } from '@/components/filters/SearchInput';
import { formatTime } from '@/lib/format';
import { AUDIT_KIND, labelOf } from '@/lib/labels';
import { AuditResultBadge, TriggerBadge } from '@/components/entities/AuditBadges';

const PAGE_SIZE = 50;
const ALL = ALL_FILTER;

function pretty(raw: string): string {
  if (!raw) return '（空）';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function Detail({ log }: { log: AuditLog }) {
  const blocks: Array<[string, string]> = [
    ['变更前', log.before],
    ['变更后', log.after],
    ['请求', log.request],
    ['响应', log.response],
  ];
  return (
    <div className="grid grid-cols-2 gap-2 bg-muted/40 p-3">
      {log.error && <div className="col-span-2 text-destructive">错误：{log.error}</div>}
      {blocks.map(([t, v]) => (
        <div key={t}>
          <div className="mb-1 text-sm text-muted-foreground">{t}</div>
          <pre className="max-h-64 overflow-auto rounded border bg-background p-2 font-mono text-sm">{pretty(v)}</pre>
        </div>
      ))}
    </div>
  );
}

const AUDIT_COLUMNS: TableColumn<AuditLog>[] = [
  numberCol<AuditLog>('at', '时间', (a) => a.at, { width: 11.75, align: 'left', cell: (a) => formatTime(a.at) }),
  textCol<AuditLog>('kind', '类型', (a) => labelOf(AUDIT_KIND, a.kind).label, { width: 6 }),
  textCol<AuditLog>('target', '对象', (a) => a.target, {
    width: 10,
    cell: (a) => <span title={a.target}>{a.target}</span>,
  }),
  textCol<AuditLog>('summary', '变更摘要', (a) => a.summary, {
    width: 28,
    cell: (a) => <span title={a.summary}>{a.summary}</span>,
  }),
  textCol<AuditLog>('trigger', '触发方式', (a) => a.trigger, {
    width: 6.5,
    cell: (a) => <TriggerBadge trigger={a.trigger} />,
  }),
  textCol<AuditLog>('result', '结果', (a) => a.result, {
    width: 6,
    cell: (a) => <AuditResultBadge result={a.result} />,
  }),
  textCol<AuditLog>('operator', '操作人', (a) => a.operator, { width: 7, cell: (a) => a.operator || '—' }),
];
const getAuditId = (a: AuditLog) => String(a.id);
const renderAuditDetail = (a: AuditLog) => <Detail log={a} />;

export default function AuditPage() {
  const [kind, setKind] = useState(ALL);
  const [trigger, setTrigger] = useState(ALL);
  const [result, setResult] = useState(ALL);
  const [q, setQ] = useState('');
  const [page, setPage] = useState(0);

  const param = (v: string) => (v === ALL ? undefined : v);
  const query = useAudit({
    kind: param(kind),
    trigger: param(trigger),
    result: param(result),
    q: q || undefined,
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
  });

  const items = useMemo(() => query.data?.items ?? [], [query.data]);
  const status = useMemo(
    () => ({
      loading: query.isLoading,
      error: query.isError ? query.error.message : null,
      empty: '没有符合条件的记录',
    }),
    [query.isLoading, query.isError, query.error],
  );
  const total = query.data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const withReset = (set: (v: string) => void) => (v: string) => {
    set(v);
    setPage(0);
  };

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      <PageHeader title="写入历史与审计" description="所有写入 NewAPI 与 nam 的记录，展开可看完整差异和原始请求/响应。" />
      <FilterBar clearable={false}>
        <FilterSelect className="w-32" value={kind} onChange={withReset(setKind)} label="类型" options={[['channel', '渠道'], ['price', '价格'], ['setting', '设置'], ['upstream', '上游'], ['cache_test', '缓存测试']]} />
        <FilterSelect className="w-32" value={trigger} onChange={withReset(setTrigger)} label="触发方式" options={[['auto', '自动'], ['manual', '手动']]} />
        <FilterSelect className="w-32" value={result} onChange={withReset(setResult)} label="结果" options={[['ok', '成功'], ['fail', '失败'], ['skipped', '已跳过']]} />
        <SearchInput
          submitOnEnter
          placeholder="搜索对象或摘要"
          value={q}
          onChange={(v) => {
            setQ(v);
            setPage(0);
          }}
        />
      </FilterBar>
      <DataTable<AuditLog>
        layoutId="audit.list"
        className="min-h-0 flex-1"
        data={items}
        columns={AUDIT_COLUMNS}
        getRowId={getAuditId}
        manualSorting
        status={status}
        renderExpanded={renderAuditDetail}
      />
      <div className="flex items-center justify-end gap-2 text-muted-foreground">
        <span>共 {total} 条，第 {page + 1} / {pages} 页</span>
        <Button size="sm" variant="outline" disabled={page === 0} onClick={() => setPage(page - 1)}>上一页</Button>
        <Button size="sm" variant="outline" disabled={page + 1 >= pages} onClick={() => setPage(page + 1)}>下一页</Button>
      </div>
    </div>
  );
}
