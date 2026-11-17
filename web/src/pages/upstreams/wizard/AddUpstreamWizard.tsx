import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { invalidate } from '@/api/invalidate';
import { Wizard, type WizardStep } from '@/components/wizard/Wizard';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useActionMutation } from '@/hooks/useActionMutation';
import { StepBasics, StepCredential, StepDetect, StepFinish } from './Steps';
import { StepTest } from './StepTest';
import { credentialOk, INITIAL_DRAFT, toCreateInput, URL_PATTERN, WIZARD_STEPS, type UpstreamDraft } from './state';

export function AddUpstreamWizard({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<UpstreamDraft>(INITIAL_DRAFT);

  useEffect(() => {
    if (open) {
      setDraft(INITIAL_DRAFT);
      setStep(0);
    }
  }, [open]);

  const patch = (next: Partial<UpstreamDraft>) => setDraft((cur) => ({ ...cur, ...next }));

  const save = useActionMutation({
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
    invalidate: (qc) => invalidate.upstream(qc),
    onSuccess: ({ created, syncNote }) => {
      if (syncNote) toast.warning(`已新增上游「${created.name}」${syncNote}`);
      else toast.success(`已新增上游「${created.name}」`);
      onOpenChange(false);
    },
  });

  const canNext = [
    draft.name.trim() !== '' && URL_PATTERN.test(draft.baseUrl.trim()),
    true,
    credentialOk(draft),
    true,
    Number(draft.rate) > 0,
  ];
  const contentOf = (index: number) => {
    if (index === 0) return <StepBasics draft={draft} onChange={patch} />;
    if (index === 1) return <StepDetect draft={draft} onChange={patch} />;
    if (index === 2) return <StepCredential draft={draft} onChange={patch} />;
    if (index === 3) return <StepTest draft={draft} />;
    return <StepFinish draft={draft} onChange={patch} />;
  };
  const steps: WizardStep[] = WIZARD_STEPS.map((title, index) => ({
    id: title,
    title,
    canNext: canNext[index] ?? false,
    content: index === step ? contentOf(index) : null,
  }));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="xl">
        <DialogHeader>
          <DialogTitle>新增上游</DialogTitle>
          <DialogDescription>只写入 nam，不会改动 NewAPI。</DialogDescription>
        </DialogHeader>
        <Wizard
          steps={steps}
          current={step}
          onCurrentChange={setStep}
          onCancel={() => onOpenChange(false)}
          onFinish={() => save.mutate()}
          finishLabel={draft.syncNow ? '保存并同步' : '保存'}
          finishing={save.isPending}
        />
      </DialogContent>
    </Dialog>
  );
}
