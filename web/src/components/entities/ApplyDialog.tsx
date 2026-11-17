import { ChevronDown, ChevronRight } from 'lucide-react';
import { useState } from 'react';
import { api } from '@/api/endpoints';
import { errorText, issuesOf, issueText } from '@/api/errors';
import { invalidate } from '@/api/invalidate';
import { useChannelPlan } from '@/api/queries';
import type { ApplyResponse, PlanItem } from '@/api/types';
import { useActionMutation } from '@/hooks/useActionMutation';
import { Button } from '@/components/ui/button';
import { Banner } from '@/components/ui/banner';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/misc';
import { showValue } from '@/lib/format';

const OP_LABELS = { create: '新建', update: '修改', delete: '删除' } as const;
const OP_TONES = { create: 'success', update: 'accent', delete: 'danger' } as const;

function PlanCard({ item, result }: { item: PlanItem; result?: ApplyResponse['results'][number] }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="rounded border">
      <div className="flex items-center gap-2 bg-muted/50 px-2 py-1">
        <Badge tone={OP_TONES[item.op]}>{OP_LABELS[item.op]}</Badge>
        <span className="font-medium">{item.name}</span>
        {result && (result.ok ? <Badge tone="success">成功</Badge> : <Badge tone="danger">失败</Badge>)}
        {result && !result.ok && <span className="truncate text-sm text-destructive" title={result.error}>{result.error}</span>}
        {item.request && (
          <button type="button" className="ml-auto inline-flex items-center text-sm text-muted-foreground hover:text-foreground" onClick={() => setOpen(!open)}>
            {open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}请求
          </button>
        )}
      </div>
      <ul className="px-2 py-1 text-base">
        {item.changes.map((c) => (
          <li key={c.field} className="flex flex-wrap items-baseline gap-1">
            <span className="w-24 shrink-0 text-muted-foreground">{c.field}</span>
            <span className="break-all">{showValue(c.old)}</span>
            <span className="text-muted-foreground">→</span>
            <span className="break-all font-medium">{showValue(c.new)}</span>
            {!c.newapi && <Badge>不写 NewAPI</Badge>}
          </li>
        ))}
        {item.changes.length === 0 && <li className="text-muted-foreground">无字段变化</li>}
      </ul>
      {open && item.request && (
        <pre className="max-h-48 overflow-auto border-t bg-muted/40 px-2 py-1 text-xs">
          {item.request.method} {item.request.path}
          {item.request.body !== undefined ? `\n${JSON.stringify(item.request.body, null, 2)}` : ''}
        </pre>
      )}
    </div>
  );
}

export function ApplyDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [outcome, setOutcome] = useState<ApplyResponse | null>(null);
  const [issueLines, setIssueLines] = useState<string[]>([]);

  const plan = useChannelPlan(open);

  const apply = useActionMutation({
    mutationFn: () => api.apply(),
    invalidate: invalidate.channels,
    onMutate: () => setIssueLines([]),
    onSuccess: (data) => setOutcome(data),
    onError: (e) => {
      const issues = issuesOf(e);
      setIssueLines(issues.length > 0 ? issues.map(issueText) : [errorText(e)]);
      return true;
    },
  });

  const close = (next: boolean) => {
    if (!next) {
      setOutcome(null);
      setIssueLines([]);
    }
    onOpenChange(next);
  };

  const items = plan.data ?? [];
  const resultOf = (item: PlanItem) => outcome?.results.find((r) => r.draft_id === item.draft_id);

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent size="3xl">
        <DialogHeader>
          <DialogTitle>应用变更</DialogTitle>
          <DialogDescription>
            {outcome
              ? `执行完成：成功 ${outcome.success} 条，失败 ${outcome.failed} 条。失败的项保留为草稿。`
              : '以下变更将串行写入 NewAPI。标注「不写 NewAPI」的字段只保存在 nam。'}
          </DialogDescription>
        </DialogHeader>
        {apply.isPending && (
          <div className="h-1.5 overflow-hidden rounded bg-muted">
            <div className="h-full w-1/3 animate-pulse rounded bg-primary" />
          </div>
        )}
        {issueLines.length > 0 && (
          <Banner tone="error">
            <ul>
              {issueLines.map((line) => <li key={line}>{line}</li>)}
            </ul>
          </Banner>
        )}
        <DialogBody className="grid content-start gap-2">
          {items.map((item) => <PlanCard key={item.draft_id} item={item} result={resultOf(item)} />)}
          {plan.isLoading && <div className="py-6 text-center text-muted-foreground">加载中…</div>}
          {plan.isError && <div className="py-6 text-center text-destructive">{plan.error.message}</div>}
          {plan.isSuccess && items.length === 0 && <div className="py-6 text-center text-muted-foreground">没有待应用的草稿</div>}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => close(false)}>{outcome ? '关闭' : '取消'}</Button>
          {!outcome && (
            <Button disabled={items.length === 0 || apply.isPending} onClick={() => apply.mutate()}>
              {apply.isPending ? '执行中…' : '确认并执行'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
