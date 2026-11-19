import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { invalidate } from '@/api/invalidate';
import { useActionMutation } from '@/hooks/useActionMutation';
import { Field } from '@/components/form/Field';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { formatDiscount } from '@/lib/format';

/** 输入文本转分数：空串为 null，非法返回 undefined。 */
function parseFraction(text: string): number | null | undefined {
  const trimmed = text.trim();
  if (trimmed === '') return null;
  const value = Number(trimmed);
  return Number.isFinite(value) && value > 0 ? value : undefined;
}

export function useModelDiscountMutation(channelId: number) {
  return useActionMutation({
    mutationFn: (input: { model: string; discount: number | null }) => api.setModelDiscount(channelId, input),
    invalidate: invalidate.prices,
  });
}

/** 模型级上游分组倍率覆盖：失焦或 Enter 提交，空值表示沿用渠道倍率。 */
export function DiscountCell({
  channelId,
  model,
  override,
  fallback,
}: {
  channelId: number;
  model: string;
  override: number | null;
  fallback: number | null;
}) {
  const mutation = useModelDiscountMutation(channelId);
  const initial = override === null ? '' : String(override);
  const [text, setText] = useState(initial);
  const [syncedFrom, setSyncedFrom] = useState(initial);
  if (syncedFrom !== initial) {
    setSyncedFrom(initial);
    setText(initial);
  }

  const commit = () => {
    if (text.trim() === initial) return;
    const discount = parseFraction(text);
    if (discount === undefined) {
      toast.error('上游分组倍率必须大于 0，留空表示沿用渠道倍率');
      setText(initial);
      return;
    }
    mutation.mutate({ model, discount });
  };

  return (
    <Input
      className="h-6 w-24 text-right tabular-nums"
      inputMode="decimal"
      value={text}
      placeholder={fallback === null ? '无法推算' : `沿用 ${formatDiscount(fallback)}`}
      disabled={mutation.isPending}
      onChange={(e) => setText(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') e.currentTarget.blur();
        if (e.key === 'Escape') setText(initial);
      }}
    />
  );
}

/** 缓存价覆盖：读 / 写相对输入价的比例。 */
export function CacheOverridePopover({ channelId }: { channelId: number }) {
  const [open, setOpen] = useState(false);
  const [readText, setReadText] = useState('');
  const [writeText, setWriteText] = useState('');
  const save = useActionMutation({
    mutationFn: (body: { read?: number | null; write?: number | null }) => api.setCacheOverride(channelId, body),
    success: '已更新缓存价覆盖',
    invalidate: invalidate.prices,
    onSuccess: () => setOpen(false),
  });

  const submit = () => {
    const read = parseFraction(readText);
    const write = parseFraction(writeText);
    if (read === undefined || write === undefined) {
      toast.error('缓存价比例必须大于 0');
      return;
    }
    const body: { read?: number | null; write?: number | null } = {};
    if (read !== null) body.read = read;
    if (write !== null) body.write = write;
    if (Object.keys(body).length === 0) {
      toast.info('请至少填写一项，或使用「清除覆盖」');
      return;
    }
    save.mutate(body);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button size="sm" variant="outline">缓存价覆盖</Button>
      </PopoverTrigger>
      <PopoverContent className="grid gap-2">
        <div className="text-sm text-muted-foreground">
          相对输入价的比例，只对本渠道生效。留空的项保持不变。
        </div>
        <Field label="缓存读比例">
          <Input inputMode="decimal" value={readText} placeholder="例如 0.1" onChange={(e) => setReadText(e.target.value)} />
        </Field>
        <Field label="缓存写比例">
          <Input inputMode="decimal" value={writeText} placeholder="例如 1.25" onChange={(e) => setWriteText(e.target.value)} />
        </Field>
        <div className="flex justify-between pt-1">
          <Button size="sm" variant="ghost" disabled={save.isPending} onClick={() => save.mutate({ read: null, write: null })}>
            清除覆盖
          </Button>
          <Button size="sm" disabled={save.isPending} onClick={submit}>保存</Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
