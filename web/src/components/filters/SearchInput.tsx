import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export interface SearchInputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  /** true：输入时只改本地草稿，按回车或点"搜索"才调用 onChange（审计页）。 */
  submitOnEnter?: boolean;
  /** 输入框宽度等，默认 w-56。 */
  className?: string;
}

export function SearchInput({ value, onChange, placeholder, submitOnEnter, className = 'w-56' }: SearchInputProps) {
  const [draft, setDraft] = useState(value);
  useEffect(() => setDraft(value), [value]);

  if (!submitOnEnter) {
    return <Input className={className} placeholder={placeholder} value={value} onChange={(e) => onChange(e.target.value)} />;
  }
  return (
    <form
      className="flex gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        onChange(draft);
      }}
    >
      <Input className={className} placeholder={placeholder} value={draft} onChange={(e) => setDraft(e.target.value)} />
      <Button type="submit" variant="outline">搜索</Button>
    </form>
  );
}
