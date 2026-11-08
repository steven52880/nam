import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import { StepBasics, StepCredential, StepDetect, StepFinish } from './Steps';
import { StepTest } from './StepTest';
import { credentialOk, INITIAL_DRAFT, toCreateInput, URL_PATTERN, WIZARD_STEPS, type UpstreamDraft } from './state';

export function AddUpstreamWizard({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const qc = useQueryClient();
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<UpstreamDraft>(INITIAL_DRAFT);

  useEffect(() => {
    if (open) {
      setDraft(INITIAL_DRAFT);
      setStep(0);
    }
  }, [open]);

  const patch = (next: Partial<UpstreamDraft>) => setDraft((cur) => ({ ...cur, ...next }));

  const save = useMutation({
    mutationFn: async () => {
      const created = await api.createUpstream(toCreateInput(draft));
      let syncNote = '';
      if (draft.syncNow) {
        try {
          const results = await api.syncUpstream(created.id, ['tokens', 'groups', 'balance']);
          const failed = results.filter((r) => !r.ok);
          if (failed.length > 0) syncNote = `，但同步有 ${failed.length} 项失败：${failed.map((r) => r.error ?? r.what).join('；')}`;
        } catch (e) {
          syncNote = `，但同步失败：${errorText(e)}`;
        }
      }
      return { created, syncNote };
    },
    onSuccess: ({ created, syncNote }) => {
      if (syncNote) toast.warning(`已新增上游「${created.name}」${syncNote}`);
      else toast.success(`已新增上游「${created.name}」`);
      void qc.invalidateQueries({ queryKey: ['upstreams'] });
      onOpenChange(false);
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  const canNext = [
    draft.name.trim() !== '' && URL_PATTERN.test(draft.baseUrl.trim()),
    true,
    credentialOk(draft),
    true,
    Number(draft.rate) > 0,
  ][step] ?? false;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>新增上游</DialogTitle>
          <DialogDescription>只写入 nam，不会改动 NewAPI。</DialogDescription>
        </DialogHeader>
        <ol className="flex flex-wrap gap-1 text-sm">
          {WIZARD_STEPS.map((title, index) => (
            <li key={title} className={cn('rounded px-2 py-0.5', index === step ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground')}>
              {index + 1}. {title}
            </li>
          ))}
        </ol>
        <div className="min-h-48">
          {step === 0 && <StepBasics draft={draft} onChange={patch} />}
          {step === 1 && <StepDetect draft={draft} onChange={patch} />}
          {step === 2 && <StepCredential draft={draft} onChange={patch} />}
          {step === 3 && <StepTest draft={draft} />}
          {step === 4 && <StepFinish draft={draft} onChange={patch} />}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
          <Button variant="outline" disabled={step === 0} onClick={() => setStep(step - 1)}>上一步</Button>
          {step < 4 ? (
            <Button disabled={!canNext} onClick={() => setStep(step + 1)}>下一步</Button>
          ) : (
            <Button disabled={!canNext || save.isPending} onClick={() => save.mutate()}>
              {draft.syncNow ? '保存并同步' : '保存'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
