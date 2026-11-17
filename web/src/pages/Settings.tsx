import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import type { JobName, Settings } from '@/api/types';
import { PageHeader } from '@/components/layout/PageHeader';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/misc';
import { useSession } from '@/hooks/useSession';
import { formatNumber, formatTime, parseGoDuration, toGoDuration, type ParsedInterval } from '@/lib/format';
import { CacheTestSection } from './settings/CacheTestSection';
import { IntervalRow, JOB_ORDER } from './settings/IntervalRow';
import { isBackfillDaysValid, RetentionFields, StatsStatusBlock } from './settings/StatsSection';
import { VendorSection } from './settings/VendorSection';

const SECTIONS = [
  ['connection', 'NewAPI 连接'],
  ['vendors', '厂商分组'],
  ['cachetest', '缓存率测试'],
  ['pricing', '定价参数'],
  ['schedule', '调度间隔'],
  ['retention', '日志保留'],
] as const;

const DEFAULT_INTERVAL: ParsedInterval = { value: 30, unit: 'm' };

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="scroll-mt-4 rounded border bg-card">
      <div className="flex h-9 items-center border-b px-3 font-medium">{title}</div>
      <div className="grid gap-3 p-3">{children}</div>
    </section>
  );
}

function Line({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <span className="w-40 shrink-0 text-muted-foreground">{label}</span>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

export default function SettingsPage() {
  const qc = useQueryClient();
  const session = useSession();
  const settings = useQuery({ queryKey: ['settings'], queryFn: api.settings });
  const backend = useQuery({ queryKey: ['backend'], queryFn: api.backend });

  const [threshold, setThreshold] = useState('50');
  const [autoWrite, setAutoWrite] = useState(false);
  const [logDays, setLogDays] = useState('30');
  const [statsDays, setStatsDays] = useState('180');
  const [backfillDays, setBackfillDays] = useState('7');
  const [logCleanup, setLogCleanup] = useState(false);
  const [intervals, setIntervals] = useState<Partial<Record<JobName, ParsedInterval>>>({});
  const loaded = useRef('');

  useEffect(() => {
    const s = settings.data;
    if (!s) return;
    const sig = JSON.stringify(s);
    if (loaded.current === sig) return;
    loaded.current = sig;
    setThreshold(String(Math.round(Number(s.protect_threshold ?? '0.5') * 10000) / 100));
    setAutoWrite(s.auto_write === 'true');
    setLogDays(s.log_retention_days ?? '30');
    setStatsDays(s.stats_retention_days ?? '180');
    setBackfillDays(s.stats_backfill_days ?? '7');
    setLogCleanup(s.newapi_log_cleanup === 'true');
    const next: Partial<Record<JobName, ParsedInterval>> = {};
    for (const j of JOB_ORDER) next[j] = parseGoDuration(s[`job_interval.${j}`]) ?? DEFAULT_INTERVAL;
    setIntervals(next);
  }, [settings.data]);

  const save = useMutation({
    mutationFn: () => {
      const body: Settings = {
        protect_threshold: String(Number(threshold) / 100),
        auto_write: String(autoWrite),
        log_retention_days: logDays,
        stats_retention_days: statsDays,
        stats_backfill_days: backfillDays,
        newapi_log_cleanup: String(logCleanup),
      };
      for (const j of JOB_ORDER) {
        const v = intervals[j];
        if (v && v.value > 0) body[`job_interval.${j}`] = toGoDuration(v);
      }
      return api.saveSettings(body);
    },
    onSuccess: (data) => {
      toast.success('设置已保存');
      qc.setQueryData(['settings'], data);
      void qc.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const thresholdOk = Number(threshold) >= 0 && threshold !== '';
  const daysOk = /^\d+$/.test(logDays) && /^\d+$/.test(statsDays) && isBackfillDaysValid(backfillDays);
  const intervalsOk = JOB_ORDER.every((j) => (intervals[j]?.value ?? 1) > 0);
  const valid = thresholdOk && daysOk && intervalsOk;
  const b = backend.data;

  return (
    <div className="flex h-full gap-4 p-4">
      <nav className="sticky top-0 hidden w-36 shrink-0 self-start md:block">
        <div className="mb-1 text-xs text-muted-foreground">分区</div>
        {SECTIONS.map(([id, label]) => (
          <a key={id} href={`#${id}`} onClick={(e) => { e.preventDefault(); document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' }); }}
            className="flex h-7 items-center rounded px-2 text-muted-foreground hover:bg-accent hover:text-foreground">{label}</a>
        ))}
        <a href="#rate" onClick={(e) => { e.preventDefault(); document.getElementById('rate')?.scrollIntoView({ behavior: 'smooth' }); }}
          className="flex h-7 items-center rounded px-2 text-muted-foreground hover:bg-accent hover:text-foreground">站点汇率</a>
      </nav>
      <div className="flex min-w-0 max-w-4xl flex-1 flex-col gap-3 overflow-y-auto pr-1">
        <PageHeader title="设置" actions={<Button disabled={!valid || save.isPending || !settings.data} onClick={() => save.mutate()}>保存</Button>} />
        <Section id="connection" title="NewAPI 连接">
          <Line label="运行模式">{session.mode === 'embedded' ? '嵌入模式' : '独立模式'}</Line>
          <Line label="NewAPI 版本">{b?.version || '—'}</Line>
          <Line label="检测到补丁">{b ? (b.patch_p1 ? '是' : '否') : '—'}</Line>
          <Line label="地址">{b?.address || '—'}</Line>
          <Line label="最近请求">{formatTime(b?.last_request_at ?? 0)}</Line>
          <Line label="最近错误">{b?.last_error || '无'}</Line>
        </Section>
        <Section id="vendors" title="厂商分组">
          <VendorSection />
        </Section>
        <Section id="cachetest" title="缓存率测试">
          <CacheTestSection />
        </Section>
        <Section id="pricing" title="定价参数">
          <Line label="变化保护阈值">
            <span className="flex items-center gap-1"><Input className="w-20 text-right" inputMode="decimal" value={threshold} onChange={(e) => setThreshold(e.target.value)} />%</span>
          </Line>
          <Line label="自动写入"><Switch checked={autoWrite} onCheckedChange={setAutoWrite} /> <span className="ml-2 text-sm text-muted-foreground">关闭后只计算，不自动写入</span></Line>
        </Section>
        <Section id="rate" title="站点汇率">
          <Line label="当前站点汇率">{formatNumber(Number(settings.data?.site_rate ?? NaN))} 元 / 1 美元站内额度</Line>
          <p className="text-sm text-muted-foreground">只读。修改站点汇率会影响全部厂商分组倍率，修改在 M5 实现。</p>
        </Section>
        <Section id="schedule" title="调度间隔">
          {JOB_ORDER.map((j) => {
            const v = intervals[j];
            return v ? <IntervalRow key={j} job={j} value={v} onChange={(nv) => setIntervals((p) => ({ ...p, [j]: nv }))} /> : null;
          })}
        </Section>
        <Section id="retention" title="日志保留">
          <RetentionFields
            logDays={logDays}
            statsDays={statsDays}
            backfillDays={backfillDays}
            logCleanup={logCleanup}
            onLogDays={setLogDays}
            onStatsDays={setStatsDays}
            onBackfillDays={setBackfillDays}
            onLogCleanup={setLogCleanup}
          />
          <StatsStatusBlock backfillDays={backfillDays} />
        </Section>
      </div>
    </div>
  );
}
