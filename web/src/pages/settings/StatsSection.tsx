import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/misc';
import { formatTime } from '@/lib/format';

const MIN_BACKFILL_DAYS = 1;
const MAX_BACKFILL_DAYS = 90;

export function isBackfillDaysValid(text: string): boolean {
  if (!/^\d+$/.test(text)) return false;
  const days = Number(text);
  return days >= MIN_BACKFILL_DAYS && days <= MAX_BACKFILL_DAYS;
}

function Line({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <span className="w-40 shrink-0 text-muted-foreground">{label}</span>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

export interface RetentionValues {
  logDays: string;
  statsDays: string;
  backfillDays: string;
  logCleanup: boolean;
}

interface RetentionFieldsProps extends RetentionValues {
  onLogDays: (value: string) => void;
  onStatsDays: (value: string) => void;
  onBackfillDays: (value: string) => void;
  onLogCleanup: (value: boolean) => void;
}

/** 「日志保留」分区的表单项；值由 Settings 页统一保存。 */
export function RetentionFields({
  logDays,
  statsDays,
  backfillDays,
  logCleanup,
  onLogDays,
  onStatsDays,
  onBackfillDays,
  onLogCleanup,
}: RetentionFieldsProps) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  return (
    <>
      <Line label="清理 NewAPI 原始日志">
        <Switch
          checked={logCleanup}
          onCheckedChange={(checked) => {
            if (checked) setConfirmOpen(true);
            else onLogCleanup(false);
          }}
        />
        <span className="ml-2 text-sm text-muted-foreground">默认关闭；开启后由 nam 定期清理 NewAPI 里已统计过的旧日志</span>
      </Line>
      <Line label="NewAPI 原始日志保留天数">
        <Input className="w-24 text-right" inputMode="numeric" value={logDays} onChange={(e) => onLogDays(e.target.value)} />
      </Line>
      <Line label="聚合保留天数">
        <Input className="w-24 text-right" inputMode="numeric" value={statsDays} onChange={(e) => onStatsDays(e.target.value)} />
      </Line>
      <Line label="首次回填天数">
        <span className="flex items-center gap-2">
          <Input className="w-24 text-right" inputMode="numeric" value={backfillDays} onChange={(e) => onBackfillDays(e.target.value)} />
          {!isBackfillDaysValid(backfillDays) && (
            <span className="text-sm text-destructive">需为 {MIN_BACKFILL_DAYS}–{MAX_BACKFILL_DAYS} 的整数</span>
          )}
        </span>
      </Line>
      <p className="text-sm text-muted-foreground">聚合保留天数为 0 表示不删除。修改后点右上角「保存」生效。</p>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="开启 NewAPI 原始日志清理"
        description="开启后会删除 NewAPI 里早于保留天数的全部类型日志（含充值、管理日志），只删除已被统计的部分，还没拉取的日志不会被删。"
        confirmLabel="开启"
        destructive
        onConfirm={() => {
          onLogCleanup(true);
          setConfirmOpen(false);
        }}
      />
    </>
  );
}

function RebuildDialog({
  open,
  onOpenChange,
  defaultDays,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  defaultDays: string;
}) {
  const qc = useQueryClient();
  const [days, setDays] = useState(defaultDays);
  const rebuild = useMutation({
    mutationFn: (value: number) => api.statsRebuild(value),
    onSuccess: () => {
      toast.success('已清空统计数据，正在后台重新统计');
      void qc.invalidateQueries({ queryKey: ['stats'] });
      onOpenChange(false);
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="清空并重新统计"
      description="会清空全部聚合数据和拉取游标，再从 NewAPI 日志里重新统计所选天数。NewAPI 里已被清理的日志无法找回。"
      confirmLabel="清空并重新统计"
      destructive
      pending={rebuild.isPending}
      disabled={!isBackfillDaysValid(days)}
      onConfirm={() => rebuild.mutate(Number(days))}
    >
      <Line label="重新统计天数">
        <span className="flex items-center gap-2">
          <Input className="w-24 text-right" inputMode="numeric" value={days} onChange={(e) => setDays(e.target.value)} />
          {!isBackfillDaysValid(days) && (
            <span className="text-sm text-destructive">需为 {MIN_BACKFILL_DAYS}–{MAX_BACKFILL_DAYS} 的整数</span>
          )}
        </span>
      </Line>
    </ConfirmDialog>
  );
}

/** 统计状态块与「清空并重新统计」。接口出错时显示「—」，不弹提示。 */
export function StatsStatusBlock({ backfillDays }: { backfillDays: string }) {
  const status = useQuery({ queryKey: ['stats', 'status'], queryFn: api.statsStatus, refetchInterval: 30_000 });
  const [rebuildOpen, setRebuildOpen] = useState(false);
  const s = status.data;
  return (
    <>
      <div className="mt-1 border-t pt-3 font-medium">统计状态</div>
      <Line label="游标">{s ? `#${s.cursor_id}` : '—'}</Line>
      <Line label="已统计到">{s ? formatTime(s.cursor_at) : '—'}</Line>
      <Line label="聚合行数">{s ? s.rows : '—'}</Line>
      <Line label="最早数据">{s ? formatTime(s.oldest_hour ?? 0) : '—'}</Line>
      <Line label="补丁 P1">{s ? (s.patch_p1 ? '是' : '否') : '—'}</Line>
      <Line label="最近拉取">{s ? s.last_pull_message || '—' : '—'}</Line>
      <div>
        <Button variant="outline" onClick={() => setRebuildOpen(true)}>清空并重新统计</Button>
      </div>
      {rebuildOpen && <RebuildDialog open onOpenChange={setRebuildOpen} defaultDays={backfillDays} />}
    </>
  );
}
