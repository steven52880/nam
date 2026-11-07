import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { JOB_TITLES, type DurationUnit, type ParsedInterval } from '@/lib/format';
import type { JobName } from '@/api/types';

const UNIT_LABELS: Record<DurationUnit, string> = { m: '分钟', h: '小时', d: '天' };

export const JOB_ORDER: JobName[] = [
  'balance', 'ratio', 'price_book', 'announcements', 'pull_logs', 'pricing', 'consistency', 'cleanup', 'backup',
];

export function IntervalRow({
  job,
  value,
  onChange,
}: {
  job: JobName;
  value: ParsedInterval;
  onChange: (v: ParsedInterval) => void;
}) {
  return (
    <div className="flex items-center gap-2">
      <span className="w-28 text-base">{JOB_TITLES[job]}</span>
      <Input
        className="w-20 text-right"
        inputMode="numeric"
        value={String(value.value)}
        onChange={(e) => {
          const n = Math.floor(Number(e.target.value));
          if (Number.isFinite(n) && n >= 0) onChange({ ...value, value: n });
        }}
      />
      <Select value={value.unit} onValueChange={(u) => onChange({ ...value, unit: u as DurationUnit })}>
        <SelectTrigger className="w-20"><SelectValue /></SelectTrigger>
        <SelectContent>
          {(Object.keys(UNIT_LABELS) as DurationUnit[]).map((u) => (
            <SelectItem key={u} value={u}>{UNIT_LABELS[u]}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
