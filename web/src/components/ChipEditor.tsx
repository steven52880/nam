import { X } from 'lucide-react';
import { useState, type ClipboardEvent, type KeyboardEvent } from 'react';
import { Input } from '@/components/ui/input';
import { splitList, uniq } from '@/lib/list';
import { cn } from '@/lib/utils';

interface ChipEditorProps {
  value: string[];
  onChange: (next: string[]) => void;
  placeholder?: string;
  /** 返回 true 的条目用警告色显示。 */
  isWarn?: (item: string) => boolean;
  className?: string;
}

/** 标签式列表编辑：回车 / 逗号添加，支持粘贴逗号分隔的列表。 */
export function ChipEditor({ value, onChange, placeholder, isWarn, className }: ChipEditorProps) {
  const [draft, setDraft] = useState('');

  const add = (text: string) => {
    const items = splitList(text);
    if (items.length > 0) onChange(uniq([...value, ...items]));
    setDraft('');
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',' || e.key === '，') {
      e.preventDefault();
      add(draft);
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  };

  const onPaste = (e: ClipboardEvent<HTMLInputElement>) => {
    const text = e.clipboardData.getData('text/plain');
    if (/[,，\n\r\t]/.test(text)) {
      e.preventDefault();
      add(draft + text);
    }
  };

  return (
    <div className={cn('rounded border border-input bg-background p-1.5', className)}>
      <div className="mb-1 flex max-h-40 flex-wrap gap-1 overflow-y-auto">
        {value.map((item) => (
          <span
            key={item}
            className={cn(
              'inline-flex h-5 items-center gap-1 rounded bg-muted pl-1.5 pr-0.5 text-sm',
              isWarn?.(item) && 'bg-warning/20 text-warning',
            )}
          >
            {item}
            <button
              type="button"
              className="rounded p-0.5 hover:bg-accent"
              title="移除"
              onClick={() => onChange(value.filter((v) => v !== item))}
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        ))}
        {value.length === 0 && <span className="text-sm text-muted-foreground">暂无条目</span>}
      </div>
      <Input
        value={draft}
        placeholder={placeholder ?? '输入后回车添加，可粘贴逗号分隔的列表'}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKeyDown}
        onPaste={onPaste}
        onBlur={() => add(draft)}
      />
    </div>
  );
}
