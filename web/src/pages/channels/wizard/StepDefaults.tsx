import { useEffect } from 'react';
import { useVendorMatches } from '@/components/model-picker/useModelGroups';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/misc';
import { allSelectedModels, isInt, type WizardState } from './state';

interface StepDefaultsProps {
  state: WizardState;
  tokenNames: Map<number, string>;
  onChange: (patch: Partial<WizardState>) => void;
}

function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-1">
      <Label>{label}</Label>
      {children}
      {error && <span className="text-xs text-destructive">{error}</span>}
    </div>
  );
}

export function StepDefaults({ state, tokenNames, onChange }: StepDefaultsProps) {
  const selectedModels = allSelectedModels(state);
  const { matches } = useVendorMatches(selectedModels);
  const vendorNames = [...new Set(selectedModels.map((m) => matches.get(m)?.vendor ?? '').filter((n) => n !== ''))];
  const defaultTestModel = (vendorName: string) =>
    selectedModels.find((m) => matches.get(m)?.vendor === vendorName) ?? '';

  const missingKey = vendorNames
    .filter((name) => state.testModels[name] === undefined && defaultTestModel(name) !== '')
    .join('|');
  useEffect(() => {
    if (!missingKey) return;
    const filled: Record<string, string> = {};
    for (const name of missingKey.split('|')) filled[name] = defaultTestModel(name);
    onChange({ testModels: { ...state.testModels, ...filled } });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [missingKey]);

  return (
    <div className="grid gap-3">
      <p className="text-sm text-muted-foreground">渠道命名规则：nam-&lt;上游&gt;-&lt;令牌&gt;-&lt;厂商&gt;。</p>
      <div className="grid grid-cols-3 gap-3">
        <Field label="优先级" error={isInt(state.priority) ? undefined : '必须是整数'}>
          <Input inputMode="numeric" value={state.priority} onChange={(e) => onChange({ priority: e.target.value })} />
        </Field>
        <Field label="权重" error={isInt(state.weight, 0) ? undefined : '必须是不小于 0 的整数'}>
          <Input inputMode="numeric" value={state.weight} onChange={(e) => onChange({ weight: e.target.value })} />
        </Field>
        <Field label="标签">
          <Input value={state.tag} onChange={(e) => onChange({ tag: e.target.value })} />
        </Field>
      </div>
      <div className="grid gap-1">
        <Label>测试模型（每个厂商一个，默认取该厂商的第一个模型）</Label>
        {vendorNames.map((name) => (
          <div key={name} className="flex items-center gap-2">
            <span className="w-24 shrink-0 text-base">{name}</span>
            <Input
              value={state.testModels[name] ?? defaultTestModel(name)}
              onChange={(e) => onChange({ testModels: { ...state.testModels, [name]: e.target.value } })}
            />
          </div>
        ))}
        {vendorNames.length === 0 && <span className="text-sm text-muted-foreground">所选模型没有归属厂商</span>}
      </div>
      <div className="grid gap-1">
        <Label>上游分组折扣（留空表示根据上游分组倍率自动推算；填写则标记为手动）</Label>
        {state.tokenIds.map((id) => (
          <div key={id} className="flex items-center gap-2">
            <span className="w-40 shrink-0 truncate text-base">{tokenNames.get(id) ?? `令牌 ${id}`}</span>
            <Input
              className="w-32"
              inputMode="decimal"
              placeholder="自动"
              value={state.discounts[id] ?? ''}
              onChange={(e) => onChange({ discounts: { ...state.discounts, [id]: e.target.value } })}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
