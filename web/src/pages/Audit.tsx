import { ChevronDown, ChevronRight } from 'lucide-react';
import { Fragment, useState } from 'react';
import { useAudit } from '@/api/queries';
import type { AuditLog } from '@/api/types';
import { PageHeader } from '@/components/layout/PageHeader';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { formatTime } from '@/lib/format';
import { AUDIT_KIND, labelOf } from '@/lib/labels';
import { AuditResultBadge, TriggerBadge } from '@/components/entities/AuditBadges';

const PAGE_SIZE = 50;
const ALL = 'all';

function pretty(raw: string): string {
  if (!raw) return '（空）';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function FilterSelect({
  value,
  onChange,
  placeholder,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  options: Array<[string, string]>;
}) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="w-32">
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{placeholder}</SelectItem>
        {options.map(([v, l]) => (
          <SelectItem key={v} value={v}>{l}</SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
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

export default function AuditPage() {
  const [kind, setKind] = useState(ALL);
  const [trigger, setTrigger] = useState(ALL);
  const [result, setResult] = useState(ALL);
  const [search, setSearch] = useState('');
  const [q, setQ] = useState('');
  const [page, setPage] = useState(0);
  const [openId, setOpenId] = useState<number | null>(null);

  const param = (v: string) => (v === ALL ? undefined : v);
  const query = useAudit({
    kind: param(kind),
    trigger: param(trigger),
    result: param(result),
    q: q || undefined,
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
  });

  const total = query.data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const withReset = (set: (v: string) => void) => (v: string) => {
    set(v);
    setPage(0);
  };

  return (
    <div className="flex flex-col gap-3 p-4">
      <PageHeader title="写入历史与审计" description="所有写入 NewAPI 与 nam 的记录，展开可看完整差异和原始请求/响应。" />
      <div className="flex flex-wrap items-center gap-2">
        <FilterSelect value={kind} onChange={withReset(setKind)} placeholder="全部类型" options={[['channel', '渠道'], ['price', '价格'], ['setting', '设置'], ['upstream', '上游'], ['cache_test', '缓存测试']]} />
        <FilterSelect value={trigger} onChange={withReset(setTrigger)} placeholder="全部触发方式" options={[['auto', '自动'], ['manual', '手动']]} />
        <FilterSelect value={result} onChange={withReset(setResult)} placeholder="全部结果" options={[['ok', '成功'], ['fail', '失败'], ['skipped', '已跳过']]} />
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            setQ(search);
            setPage(0);
          }}
        >
          <Input className="w-56" placeholder="搜索对象或摘要" value={search} onChange={(e) => setSearch(e.target.value)} />
          <Button type="submit" variant="outline">搜索</Button>
        </form>
      </div>
      <div className="overflow-hidden rounded border bg-card">
        <table className="w-full text-base">
          <thead className="bg-muted text-left text-sm text-muted-foreground">
            <tr>
              <th className="w-8" />
              {['时间', '类型', '对象', '变更摘要', '触发方式', '结果', '操作人'].map((h) => (
                <th key={h} className="h-7 px-2 font-medium">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(query.data?.items ?? []).map((a) => {
              const open = openId === a.id;
              return (
                <Fragment key={a.id}>
                  <tr className="cursor-pointer border-t hover:bg-accent/40" onClick={() => setOpenId(open ? null : a.id)}>
                    <td className="pl-2">{open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}</td>
                    <td className="h-7 whitespace-nowrap px-2 tabular-nums">{formatTime(a.at)}</td>
                    <td className="px-2">{labelOf(AUDIT_KIND, a.kind).label}</td>
                    <td className="max-w-40 truncate px-2" title={a.target}>{a.target}</td>
                    <td className="max-w-md truncate px-2" title={a.summary}>{a.summary}</td>
                    <td className="px-2"><TriggerBadge trigger={a.trigger} /></td>
                    <td className="px-2"><AuditResultBadge result={a.result} /></td>
                    <td className="px-2">{a.operator || '—'}</td>
                  </tr>
                  {open && (
                    <tr className="border-t">
                      <td colSpan={8} className="p-0"><Detail log={a} /></td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
            {query.data?.items.length === 0 && (
              <tr><td colSpan={8} className="h-16 text-center text-muted-foreground">没有符合条件的记录</td></tr>
            )}
            {query.isError && (
              <tr><td colSpan={8} className="h-16 text-center text-destructive">{query.error.message}</td></tr>
            )}
          </tbody>
        </table>
      </div>
      <div className="flex items-center justify-end gap-2 text-muted-foreground">
        <span>共 {total} 条，第 {page + 1} / {pages} 页</span>
        <Button size="sm" variant="outline" disabled={page === 0} onClick={() => setPage(page - 1)}>上一页</Button>
        <Button size="sm" variant="outline" disabled={page + 1 >= pages} onClick={() => setPage(page + 1)}>下一页</Button>
      </div>
    </div>
  );
}
