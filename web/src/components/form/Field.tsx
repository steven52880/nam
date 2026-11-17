import type { ReactNode } from 'react';
import { Label } from '@/components/ui/misc';

export interface FieldProps {
  label: ReactNode;
  /** 灰色说明。 */
  hint?: ReactNode;
  /** 红色报错，可以和 hint 同时出现。 */
  error?: ReactNode;
  required?: boolean;
  /** 标签在左、控件在右（设置页）。 */
  inline?: boolean;
  /** 需要关联控件时由调用方传入控件的 id。 */
  htmlFor?: string;
  /** 可以是多个元素（如输入框 + 按钮）。 */
  children: ReactNode;
}

export function Field({ label, hint, error, required, inline, htmlFor, children }: FieldProps) {
  if (inline) {
    return (
      <div className="flex items-center gap-3">
        <Label htmlFor={htmlFor} className="w-40 shrink-0 text-base font-normal">
          {label}
          {required && <span className="text-destructive"> *</span>}
        </Label>
        <div className="min-w-0 flex-1">
          {children}
          {hint && <span className="mt-1 block text-xs text-muted-foreground">{hint}</span>}
          {error && <span className="mt-1 block text-xs text-destructive">{error}</span>}
        </div>
      </div>
    );
  }
  return (
    <div className="grid gap-1">
      <Label htmlFor={htmlFor}>
        {label}
        {required && <span className="text-destructive"> *</span>}
      </Label>
      {children}
      {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
      {error && <span className="text-xs text-destructive">{error}</span>}
    </div>
  );
}
