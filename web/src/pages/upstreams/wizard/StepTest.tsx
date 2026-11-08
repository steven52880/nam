import { useMutation } from '@tanstack/react-query';
import { useEffect } from 'react';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { CheckResult } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { toTestInput, type UpstreamDraft } from './state';

export function CheckList({ results, pending, error }: { results: CheckResult[]; pending: boolean; error?: string }) {
  return (
    <ul className="grid gap-1">
      {results.map((r) => (
        <li key={r.name} className="flex items-center gap-2 rounded border px-2 py-1">
          {r.skipped ? <Badge>跳过</Badge> : r.ok ? <Badge tone="success">通过</Badge> : <Badge tone="danger">失败</Badge>}
          <span className="font-medium">{r.name}</span>
          <span className="min-w-0 flex-1 truncate text-sm text-muted-foreground" title={r.message}>{r.message}</span>
          <span className="text-sm tabular-nums text-muted-foreground">{r.ms} 毫秒</span>
        </li>
      ))}
      {pending && <li className="py-4 text-center text-muted-foreground">测试中，最长约 2 分钟…</li>}
      {error && <li className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">{error}</li>}
    </ul>
  );
}

/** 第 4 步：进入时自动测试一次，可以重新测试。 */
export function StepTest({ draft }: { draft: UpstreamDraft }) {
  const test = useMutation({ mutationFn: () => api.testUpstreamInput(toTestInput(draft)) });

  useEffect(() => {
    test.mutate();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const failed = (test.data ?? []).some((r) => !r.ok && !r.skipped);
  return (
    <div className="grid gap-2">
      <div className="flex items-center gap-2">
        <Button size="sm" variant="outline" disabled={test.isPending} onClick={() => test.mutate()}>重新测试</Button>
        {failed && <span className="text-sm text-destructive">有检查未通过，可以返回上一步修改凭证；也可以忽略并继续保存。</span>}
      </div>
      <CheckList
        results={test.data ?? []}
        pending={test.isPending}
        error={test.isError ? errorText(test.error) : undefined}
      />
    </div>
  );
}
