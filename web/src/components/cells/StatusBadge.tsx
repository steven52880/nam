import { Badge } from '@/components/ui/misc';
import { labelOf, type LabelDef } from '@/lib/labels';

/** 徽标：文字和颜色都来自 lib/labels.ts 的字典；字典里没有的值显示原值、灰色。 */
export function StatusBadge({ dict, value }: { dict: Readonly<Record<string | number, LabelDef>>; value: string | number }) {
  const def = labelOf(dict, value);
  return <Badge tone={def.tone} title={def.hint}>{def.label}</Badge>;
}
