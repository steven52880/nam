import { useState } from 'react';
import type { StatsWindow } from '@/api/types';
import { Field } from '@/components/form/Field';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { PopoverContent } from '@/components/ui/popover';
import {
  MAX_WINDOW_DAYS,
  MAX_WINDOW_HOURS,
  buildStatsWindow,
  parseCustomWindow,
  validateWindowInput,
  type StatsWindowUnit,
} from '@/lib/statsWindow';

const UNITS: ReadonlyArray<[StatsWindowUnit, string]> = [
  ['h', '小时'],
  ['d', '天'],
];

/** 自定义统计时长的浮层内容：数字 + 单位 + 确定。每次打开重新挂载，用当前值预填。 */
export function CustomWindowContent({
  current,
  onSubmit,
}: {
  current: StatsWindow;
  onSubmit: (window: StatsWindow) => void;
}) {
  const initial = parseCustomWindow(current);
  const [text, setText] = useState(initial ? String(initial.amount) : '');
  const [unit, setUnit] = useState<StatsWindowUnit>(initial?.unit ?? 'h');
  const error = validateWindowInput(text, unit);
  // 空输入不提示，等用户输入后再报错
  const shownError = text.trim() === '' ? null : error;

  const submit = () => {
    if (error) return;
    const next = buildStatsWindow(Number(text.trim()), unit);
    if (next) onSubmit(next);
  };

  return (
    <PopoverContent className="grid w-64 gap-2" align="end">
      <Field
        label="自定义统计时长"
        hint={`1–${MAX_WINDOW_HOURS} 小时，或 1–${MAX_WINDOW_DAYS} 天`}
        error={shownError}
      >
        <div className="flex items-center gap-1">
          <Input
            autoFocus
            inputMode="numeric"
            className="w-20 text-right tabular-nums"
            value={text}
            placeholder="例如 12"
            aria-invalid={shownError ? true : undefined}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                submit();
              }
            }}
          />
          {UNITS.map(([value, label]) => (
            <Button
              key={value}
              type="button"
              size="sm"
              variant={unit === value ? 'default' : 'outline'}
              aria-pressed={unit === value}
              onClick={() => setUnit(value)}
            >
              {label}
            </Button>
          ))}
        </div>
      </Field>
      <div className="flex justify-end pt-1">
        <Button size="sm" disabled={error !== null} onClick={submit}>确定</Button>
      </div>
    </PopoverContent>
  );
}
