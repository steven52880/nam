import type { UseMutationResult } from '@tanstack/react-query';
import { errorText } from '@/api/errors';
import type { BatchPreview, BatchRequest } from '@/api/types';
import { Badge } from '@/components/ui/misc';
import { formatNumber } from '@/lib/format';
import { cn } from '@/lib/utils';

export function StepPreview({ preview }: { preview: UseMutationResult<BatchPreview, Error, BatchRequest> }) {
  const items = preview.data?.items ?? [];
  const unassigned = preview.data?.unassigned ?? [];
  const duplicates = items.filter((i) => i.duplicate).length;

  return (
    <div className="grid gap-2">
      {preview.isPending && <div className="py-6 text-center text-muted-foreground">生成预览中…</div>}
      {preview.isError && <div className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">{errorText(preview.error)}</div>}
      {unassigned.length > 0 && (
        <div className="rounded bg-warning/15 px-2 py-1 text-sm text-warning">
          以下模型不属于任何厂商，不会生成渠道：{unassigned.join('，')}
        </div>
      )}
      {duplicates > 0 && (
        <div className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">
          有 {duplicates} 个渠道名称重复（标红），请返回修改后再加入草稿。
        </div>
      )}
      {preview.isSuccess && (
        <div className="max-h-80 overflow-auto rounded border">
          <table className="w-max min-w-full text-base">
            <thead className="sticky top-0 bg-muted text-left text-sm text-muted-foreground">
              <tr>
                {['名称', '令牌', '厂商', '分组', '渠道类型', '模型', '上游分组折扣', '优先级', '权重', '测试模型'].map((h) => (
                  <th key={h} className="h-7 whitespace-nowrap px-2 font-medium">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <tr key={it.name} className="h-7 border-t">
                  <td className={cn('whitespace-nowrap px-2 font-medium', it.duplicate && 'text-destructive')}>
                    {it.name}{it.duplicate && <Badge tone="danger" className="ml-1">重名</Badge>}
                  </td>
                  <td className="px-2">{it.token_name}</td>
                  <td className="px-2">{it.vendor}</td>
                  <td className="px-2">{it.group}</td>
                  <td className="whitespace-nowrap px-2">{it.channel_type === 24 ? '24 Gemini 原生' : '58 透传'}</td>
                  <td className="px-2" title={it.fields.models?.join('，')}>{it.fields.models?.length ?? 0} 个</td>
                  <td className="px-2 text-right tabular-nums">
                    {it.ext.discount !== undefined ? formatNumber(it.ext.discount, 3) : '—'}
                    {it.ext.discount_source === 'manual' && <Badge tone="warning" className="ml-1">手动</Badge>}
                  </td>
                  <td className="px-2 text-right tabular-nums">{it.fields.priority ?? '—'}</td>
                  <td className="px-2 text-right tabular-nums">{it.fields.weight ?? '—'}</td>
                  <td className="px-2">{it.fields.test_model || '—'}</td>
                </tr>
              ))}
              {items.length === 0 && <tr><td colSpan={10} className="h-12 text-center text-muted-foreground">没有可生成的渠道</td></tr>}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
