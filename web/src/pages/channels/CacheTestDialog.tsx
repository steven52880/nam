import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { invalidate } from '@/api/invalidate';
import { useCacheTest } from '@/api/queries';
import type { CacheTest, CacheTestRound, ChannelRow } from '@/api/types';
import { useActionMutation } from '@/hooks/useActionMutation';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { formatPercent } from '@/lib/format';
import { Label } from '@/components/ui/misc';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

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

function RoundsTable({ test }: { test: CacheTest }) {
  const rounds = parseRounds(test.per_round);
  const running = test.status === 'running';
  return (
    <div className="grid gap-2">
      {(rounds.length > 0 || running) && (
        <table className="w-full text-sm tabular-nums">
          <thead>
            <tr className="border-b text-right text-muted-foreground">
              <th className="px-2 py-1 text-left font-normal">轮次</th>
              <th className="px-2 py-1 font-normal">输入</th>
              <th className="px-2 py-1 font-normal">缓存读取</th>
              <th className="px-2 py-1 font-normal">缓存写入</th>
              <th className="px-2 py-1 font-normal">输出</th>
              <th className="px-2 py-1 font-normal">耗时（毫秒）</th>
            </tr>
          </thead>
          <tbody>
            {rounds.map((r) => (
              <tr key={r.round} className="border-b text-right last:border-0">
                <td className="px-2 py-1 text-left">{r.round}</td>
                <td className="px-2 py-1">{r.input}</td>
                <td className="px-2 py-1">{r.cache_read}</td>
                <td className="px-2 py-1">{r.cache_write}</td>
                <td className="px-2 py-1">{r.output}</td>
                <td className="px-2 py-1">{r.duration_ms}</td>
              </tr>
            ))}
            {running && rounds.length < test.rounds && (
              <tr className="text-muted-foreground">
                <td colSpan={6} className="px-2 py-1 text-left">第 {rounds.length + 1} 轮进行中…</td>
              </tr>
            )}
          </tbody>
        </table>
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
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>测缓存：{row?.name ?? ''}</DialogTitle>
          <DialogDescription>由 nam 直接请求上游（不经过 NewAPI，不会出现在 NewAPI 日志中），会真实计费；渠道在 NewAPI 中的模型映射、参数覆盖不生效。</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-[1fr_6rem] gap-3">
          <div className="grid gap-1">
            <Label>测试模型</Label>
            <Select value={model} onValueChange={setModel} disabled={running}>
              <SelectTrigger><SelectValue placeholder="选择模型" /></SelectTrigger>
              <SelectContent>
                {row?.models.map((m) => <SelectItem key={m} value={m}>{m}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-1">
            <Label>轮数（2~10）</Label>
            <Input inputMode="numeric" value={rounds} disabled={running} onChange={(e) => setRounds(e.target.value)} />
          </div>
        </div>
        {test && <RoundsTable test={test} />}
        {poll.isError && <div className="text-sm text-destructive">{errorText(poll.error)}</div>}
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
