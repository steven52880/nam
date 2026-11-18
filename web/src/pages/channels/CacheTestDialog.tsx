import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { invalidate } from '@/api/invalidate';
import { useCacheTest } from '@/api/queries';
import type { CacheTest, CacheTestRound, ChannelRow } from '@/api/types';
import { useActionMutation } from '@/hooks/useActionMutation';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/form/Field';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { DataTable, numberCol, type SortSpec, type TableColumn } from '@/components/table';
import { Input } from '@/components/ui/input';
import { formatPercent } from '@/lib/format';
import { Combobox, type ComboboxItem } from '@/components/ui/combobox';

function isRound(v: unknown): v is CacheTestRound {
  return typeof v === 'object' && v !== null && 'round' in v && 'input' in v;
}

function parseRounds(raw: string): CacheTestRound[] {
  if (!raw) return [];
  try {
    const data: unknown = JSON.parse(raw);
    return Array.isArray(data) ? data.filter(isRound) : [];
  } catch {
    return [];
  }
}

const ROUND_SORT: SortSpec[] = [{ id: 'round', desc: false }];

const ROUND_COLUMNS: TableColumn<CacheTestRound>[] = [
  numberCol('round', '轮次', (r) => r.round, { width: 4.5, align: 'left' }),
  numberCol('input', '输入', (r) => r.input, { width: 4.5 }),
  numberCol('cache_read', '缓存读取', (r) => r.cache_read, { width: 5.5 }),
  numberCol('cache_write', '缓存写入', (r) => r.cache_write, { width: 5.5 }),
  numberCol('output', '输出', (r) => r.output, { width: 4.5 }),
  numberCol('duration_ms', '耗时（毫秒）', (r) => r.duration_ms, { width: 7.5 }),
];

function roundId(r: CacheTestRound): string {
  return String(r.round);
}

function RoundsTable({ test }: { test: CacheTest }) {
  const rounds = useMemo(() => parseRounds(test.per_round), [test.per_round]);
  const running = test.status === 'running';
  const pendingText = running && rounds.length < test.rounds ? `第 ${rounds.length + 1} 轮进行中…` : null;
  return (
    <div className="grid gap-2">
      {(rounds.length > 0 || running) && (
        <DataTable<CacheTestRound>
          layoutId="channels.cache-rounds"
          className="max-h-72"
          data={rounds}
          columns={ROUND_COLUMNS}
          getRowId={roundId}
          defaultSort={ROUND_SORT}
          toolbar={rounds.length > 0 && pendingText ? <span className="text-sm text-muted-foreground">{pendingText}</span> : undefined}
          status={{ empty: pendingText ?? undefined }}
        />
      )}
      {running && <div className="text-sm text-muted-foreground">测试中…每 2 秒刷新一次</div>}
      {running && test.rate !== null && (
        <div className="text-sm">
          缓存率：<span className="font-medium">{formatPercent(test.rate)}</span>
          <span className="ml-2 text-muted-foreground">（暂定，已完成 {rounds.length} / {test.rounds} 轮）</span>
        </div>
      )}
      {test.status === 'ok' && (
        <div className="text-sm">
          缓存率：<span className="font-medium">{formatPercent(test.rate)}</span>
          <span className="ml-2 text-muted-foreground">（统计第 2 轮起；{test.model}，{test.rounds} 轮）</span>
        </div>
      )}
      {test.error && <div className="text-sm text-destructive">{test.error}</div>}
    </div>
  );
}

export function CacheTestDialog({ row, onClose }: { row: ChannelRow | null; onClose: () => void }) {
  const qc = useQueryClient();
  const [model, setModel] = useState('');
  const [rounds, setRounds] = useState('3');
  const [testId, setTestId] = useState<number | null>(null);
  const notified = useRef<number | null>(null);
  const modelItems = useMemo<ComboboxItem[]>(() => (row?.models ?? []).map((m) => ({ value: m, label: m })), [row]);

  useEffect(() => {
    setModel(row?.models[0] ?? '');
    setRounds('3');
    setTestId(null);
    notified.current = null;
  }, [row]);

  const start = useActionMutation({
    mutationFn: () => api.startCacheTest(row?.id ?? 0, { model, rounds: Number(rounds), mode: 'direct' }),
    invalidate: invalidate.cacheTests,
    onSuccess: (res) => setTestId(res.test_id),
  });

  const poll = useCacheTest(row !== null ? testId : null);
  const test = poll.data;

  useEffect(() => {
    if (!test || test.status === 'running' || notified.current === test.id) return;
    notified.current = test.id;
    void invalidate.cacheTests(qc);
    if (test.status === 'ok') {
      const rate = formatPercent(test.rate);
      toast.success(`缓存率 ${rate}（${test.model}，${test.rounds} 轮）`);
    } else {
      toast.error(`缓存测试失败：${test.error || '未知错误'}`);
    }
  }, [test, qc]);

  const roundsNum = Number(rounds);
  const roundsOk = Number.isInteger(roundsNum) && roundsNum >= 2 && roundsNum <= 10;
  const running = start.isPending || test?.status === 'running';
  const close = () => {
    void invalidate.cacheTests(qc);
    onClose();
  };

  return (
    <Dialog open={row !== null} onOpenChange={(open) => { if (!open) close(); }}>
      <DialogContent size="xl">
        <DialogHeader>
          <DialogTitle>测缓存：{row?.name ?? ''}</DialogTitle>
          <DialogDescription>由 nam 直接请求上游（不经过 NewAPI，不会出现在 NewAPI 日志中），会真实计费；渠道在 NewAPI 中的模型映射、参数覆盖不生效。</DialogDescription>
        </DialogHeader>
        <DialogBody className="grid content-start gap-3">
          <div className="grid grid-cols-[1fr_6rem] gap-3">
            <Field label="测试模型">
              <Combobox
                variant="field"
                items={modelItems}
                value={model === '' ? null : model}
                onChange={(v) => setModel(v ?? '')}
                disabled={running}
                placeholder="选择模型"
              />
            </Field>
            <Field label="轮数（2~10）">
              <Input inputMode="numeric" value={rounds} disabled={running} onChange={(e) => setRounds(e.target.value)} />
            </Field>
          </div>
          {test && <RoundsTable test={test} />}
          {poll.isError && <div className="text-sm text-destructive">{errorText(poll.error)}</div>}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={close}>关闭</Button>
          <Button disabled={running || model === '' || !roundsOk} onClick={() => start.mutate()}>
            {running ? '测试中…' : test ? '重新测试' : '开始测试'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
