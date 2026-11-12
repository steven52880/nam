import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import { CHANNELS_KEY } from '../useChannels';
import { StepDefaults } from './StepDefaults';
import { StepPreview } from './StepPreview';
import { StepTokenModels } from './StepTokenModels';
import { StepTokens } from './StepTokens';
import { StepUpstream } from './StepUpstream';
import { buildBatchRequest, INITIAL_WIZARD, isInt, STEP_TITLES, type WizardState } from './state';

interface BatchWizardProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** 从上游抽屉进入时预选的上游；此时直接从第 2 步开始。 */
  presetUpstreamId?: number | null;
  /** 从上游抽屉的令牌行进入时预选的令牌。 */
  presetTokenId?: number | null;
}

export function BatchWizard({ open, onOpenChange, presetUpstreamId, presetTokenId }: BatchWizardProps) {
  const qc = useQueryClient();
  const [step, setStep] = useState(0);
  const [state, setState] = useState<WizardState>(INITIAL_WIZARD);

  useEffect(() => {
    if (!open) return;
    if (presetUpstreamId) {
      setState({ ...INITIAL_WIZARD, upstreamId: presetUpstreamId, tokenIds: presetTokenId ? [presetTokenId] : [] });
      setStep(1);
    } else {
      setState(INITIAL_WIZARD);
      setStep(0);
    }
  }, [open, presetUpstreamId, presetTokenId]);

  const patch = (next: Partial<WizardState>) => setState((cur) => ({ ...cur, ...next }));
  const tokens = useQuery({
    queryKey: ['upstream-tokens', state.upstreamId],
    queryFn: () => api.upstreamTokens(state.upstreamId ?? 0),
    enabled: state.upstreamId !== null,
  });
  const tokenNames = useMemo(() => new Map((tokens.data ?? []).map((t) => [t.id, t.name])), [tokens.data]);

  // 新选中的令牌，把它已保存的模型列表作为初始值。
  useEffect(() => {
    const data = tokens.data;
    if (!data) return;
    const fill: Record<number, string[]> = {};
    for (const id of state.tokenIds) {
      const token = data.find((t) => t.id === id);
      if (token && state.tokenModels[id] === undefined) fill[id] = token.models ?? [];
    }
    if (Object.keys(fill).length > 0) setState((cur) => ({ ...cur, tokenModels: { ...fill, ...cur.tokenModels } }));
  }, [tokens.data, state.tokenIds, state.tokenModels]);

  const preview = useMutation({ mutationFn: api.batchPreview, onError: (e: Error) => toast.error(errorText(e)) });
  const create = useMutation({
    mutationFn: () => {
      const specs = (preview.data?.items ?? []).map((i) => ({ fields: i.fields, ext: i.ext }));
      return api.batchCreate(specs);
    },
    onSuccess: ({ ids }) => {
      toast.success(`已将 ${ids.length} 个新渠道加入草稿，请到「应用变更」确认`);
      void qc.invalidateQueries({ queryKey: CHANNELS_KEY });
      onOpenChange(false);
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  const request = buildBatchRequest(state);
  useEffect(() => {
    if (step === 4 && request) preview.mutate(request);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step]);

  const canNext = [
    state.upstreamId !== null,
    state.tokenIds.length > 0,
    state.tokenIds.length > 0 && state.tokenIds.every((id) => (state.tokenModels[id] ?? []).length > 0),
    isInt(state.priority) && isInt(state.weight, 0),
  ][step] ?? false;
  const previewItems = preview.data?.items ?? [];
  const canCreate = previewItems.length > 0 && previewItems.every((i) => !i.duplicate) && !create.isPending;

  const body = () => {
    if (step === 0) return <StepUpstream value={state.upstreamId} onChange={(id) => patch({ upstreamId: id, tokenIds: [], tokenModels: {} })} />;
    if (state.upstreamId === null) return null;
    if (step === 1) return <StepTokens upstreamId={state.upstreamId} value={state.tokenIds} onChange={(ids) => patch({ tokenIds: ids })} />;
    if (step === 2) {
      return (
        <StepTokenModels
          upstreamId={state.upstreamId}
          tokens={tokens.data ?? []}
          tokenIds={state.tokenIds}
          value={state.tokenModels}
          onChange={(id, models) => setState((cur) => ({ ...cur, tokenModels: { ...cur.tokenModels, [id]: models } }))}
        />
      );
    }
    if (step === 3) return <StepDefaults state={state} tokenNames={tokenNames} onChange={patch} />;
    return <StepPreview preview={preview} tokenNames={tokenNames} />;
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92vh] max-w-4xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>批量创建渠道</DialogTitle>
          <DialogDescription>按令牌和厂商拆分生成渠道，只进入草稿，不会立即写入 NewAPI。</DialogDescription>
        </DialogHeader>
        <ol className="flex gap-1 text-sm">
          {STEP_TITLES.map((title, index) => (
            <li
              key={title}
              className={cn(
                'rounded px-2 py-0.5',
                index === step ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground',
              )}
            >
              {title}
            </li>
          ))}
        </ol>
        <div className="min-h-64">{body()}</div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
          <Button variant="outline" disabled={step === 0 || (step === 1 && !!presetUpstreamId)} onClick={() => setStep(step - 1)}>上一步</Button>
          {step < 4 ? (
            <Button disabled={!canNext} onClick={() => setStep(step + 1)}>下一步</Button>
          ) : (
            <Button disabled={!canCreate} onClick={() => create.mutate()}>加入草稿</Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
