import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Play } from 'lucide-react';
import { Link } from 'react-router-dom';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { PageHeader } from '@/components/layout/PageHeader';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { formatDuration, formatIntervalSeconds, formatTime, JOB_TITLES } from '@/lib/format';
import type { JobStatus } from '@/api/types';
import { AuditResultBadge } from '@/pages/audit/AuditBadges';

function JobResult({ job }: { job: JobStatus }) {
  if (job.running) return <Badge tone="accent">运行中</Badge>;
  if (job.queued) return <Badge tone="warning">排队中</Badge>;
  if (job.last_result === 'ok') return <Badge tone="success">成功</Badge>;
  if (job.last_result === 'fail') return <Badge tone="danger">失败</Badge>;
  return <Badge>未运行</Badge>;
}

function JobsTable() {
  const qc = useQueryClient();
  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs, refetchInterval: 10000 });
  const run = useMutation({
    mutationFn: (name: string) => api.runJob(name),
    onSuccess: (_, name) => {
      toast.success(`已将「${JOB_TITLES[name as keyof typeof JOB_TITLES] ?? name}」加入队列`);
      void qc.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <section className="rounded border bg-card">
      <div className="flex h-9 items-center border-b px-3 font-medium">后台任务</div>
      <table className="w-full text-base">
        <thead className="bg-muted text-left text-sm text-muted-foreground">
          <tr>
            {['名称', '间隔', '上次运行', '耗时', '结果', '说明', ''].map((h) => (
              <th key={h} className="h-7 px-3 font-medium">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(jobs.data ?? []).map((j) => (
            <tr key={j.name} className="border-t hover:bg-accent/40">
              <td className="h-7 px-3">{j.title || JOB_TITLES[j.name]}</td>
              <td className="px-3">{formatIntervalSeconds(j.interval_s)}</td>
              <td className="px-3 tabular-nums">{formatTime(j.last_start)}</td>
              <td className="px-3 tabular-nums">{formatDuration(j.last_duration_ms)}</td>
              <td className="px-3"><JobResult job={j} /></td>
              <td className="max-w-xs truncate px-3 text-muted-foreground" title={j.last_message}>{j.last_message || '—'}</td>
              <td className="px-3 text-right">
                <Button size="sm" variant="outline" disabled={run.isPending || j.running} onClick={() => run.mutate(j.name)}>
                  <Play className="h-3 w-3" />立即运行
                </Button>
              </td>
            </tr>
          ))}
          {jobs.isLoading && <tr><td colSpan={7} className="h-12 text-center text-muted-foreground">加载中…</td></tr>}
          {jobs.isError && <tr><td colSpan={7} className="h-12 text-center text-destructive">{jobs.error.message}</td></tr>}
        </tbody>
      </table>
    </section>
  );
}

function RecentAudit() {
  const q = useQuery({ queryKey: ['audit', 'recent'], queryFn: () => api.audit({ limit: 5 }) });
  return (
    <section className="rounded border bg-card">
      <div className="flex h-9 items-center justify-between border-b px-3">
        <span className="font-medium">最近 5 条写入</span>
        <Link to="/audit" className="text-sm text-primary hover:underline">全部写入历史</Link>
      </div>
      <ul>
        {(q.data?.items ?? []).map((a) => (
          <li key={a.id} className="flex h-7 items-center gap-3 border-t px-3 first:border-t-0">
            <span className="w-36 shrink-0 tabular-nums text-muted-foreground">{formatTime(a.at)}</span>
            <span className="w-24 shrink-0 truncate">{a.target}</span>
            <span className="min-w-0 flex-1 truncate">{a.summary}</span>
            <AuditResultBadge result={a.result} />
          </li>
        ))}
        {q.data?.items.length === 0 && <li className="flex h-12 items-center justify-center text-muted-foreground">暂无写入记录</li>}
        {q.isLoading && <li className="flex h-12 items-center justify-center text-muted-foreground">加载中…</li>}
        {q.isError && <li className="flex h-12 items-center justify-center text-destructive">{q.error.message}</li>}
      </ul>
    </section>
  );
}

const PLACEHOLDER_CARDS = ['告警卡片', '今日指标', '余额汇总'];

export default function OverviewPage() {
  return (
    <div className="flex flex-col gap-3 p-4">
      <PageHeader title="总览" description="后台任务与最近写入；告警、指标和余额汇总将在后续里程碑加入。" />
      <div className="grid grid-cols-3 gap-3">
        {PLACEHOLDER_CARDS.map((t) => (
          <div key={t} className="flex h-20 flex-col justify-center rounded border border-dashed px-3">
            <div className="font-medium">{t}</div>
            <div className="text-sm text-muted-foreground">后续里程碑实现</div>
          </div>
        ))}
      </div>
      <JobsTable />
      <RecentAudit />
    </div>
  );
}
