import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { DialogBody, DialogFooter } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';

export interface WizardStep {
  id: string;
  title: string;
  /** 当前步骤是否允许点"下一步"。 */
  canNext: boolean;
  content: ReactNode;
}

export interface WizardProps {
  steps: WizardStep[];
  current: number;
  onCurrentChange: (index: number) => void;
  onCancel: () => void;
  onFinish: () => void;
  finishLabel: string;
  finishing?: boolean;
  finishDisabled?: boolean;
  /** 第一个可返回的步骤（默认 0）；从上游预选进入时可以设为 1。 */
  minStep?: number;
}

/** 只负责步骤条、内容区和底部按钮；放在 DialogContent 里使用，状态由调用方持有。 */
export function Wizard({
  steps,
  current,
  onCurrentChange,
  onCancel,
  onFinish,
  finishLabel,
  finishing,
  finishDisabled,
  minStep = 0,
}: WizardProps) {
  const step = steps[current];
  const last = current === steps.length - 1;
  return (
    <>
      <ol className="flex flex-wrap gap-1 text-sm">
        {steps.map((s, index) => (
          <li
            key={s.id}
            className={cn(
              'rounded px-2 py-0.5',
              index === current ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground',
            )}
          >
            {index + 1}. {s.title}
          </li>
        ))}
      </ol>
      <DialogBody>
        <div className="min-h-48">{step?.content}</div>
      </DialogBody>
      <DialogFooter>
        <Button variant="outline" onClick={onCancel}>取消</Button>
        <Button variant="outline" disabled={current <= minStep} onClick={() => onCurrentChange(current - 1)}>上一步</Button>
        {last ? (
          <Button disabled={!step?.canNext || finishing || finishDisabled} onClick={onFinish}>{finishLabel}</Button>
        ) : (
          <Button disabled={!step?.canNext} onClick={() => onCurrentChange(current + 1)}>下一步</Button>
        )}
      </DialogFooter>
    </>
  );
}
