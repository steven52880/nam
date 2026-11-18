import type { UseMutationResult } from '@tanstack/react-query';
import { errorText } from '@/api/errors';
import type { BatchPreview, BatchPreviewItem, BatchRequest } from '@/api/types';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { DataTable, customCol, numberCol, textCol, type TableColumn } from '@/components/table';
import { Banner } from '@/components/ui/banner';
import { Badge } from '@/components/ui/misc';
import { DASH, formatNumber } from '@/lib/format';
import { cn } from '@/lib/utils';

const NO_ITEMS: BatchPreviewItem[] = [];
const PREVIEW_STATUS = { empty: '没有可生成的渠道' };

function previewRowId(it: BatchPreviewItem): string {
  return it.name;
}

const PREVIEW_COLUMNS: TableColumn<BatchPreviewItem>[] = [
  textCol('name', '名称', (it) => it.name, {
    width: 14,
    cell: (it) => (
      <span className={cn('font-medium', it.duplicate && 'text-destructive')}>
        {it.name}{it.duplicate && <Badge tone="danger" className="ml-1">重名</Badge>}
      </span>
    ),
  }),
  textCol('token_name', '令牌', (it) => it.token_name, { width: 8 }),
  textCol('vendor', '厂商', (it) => it.vendor, { width: 7 }),
  textCol('group', '分组', (it) => it.group, { width: 8 }),
  textCol('channel_type', '渠道类型', (it) => (it.channel_type === 24 ? '24 Gemini 原生' : '58 透传'), { width: 9.5 }),
  customCol<BatchPreviewItem>('models', '模型', {
    width: 5,
    accessor: (it) => it.fields.models?.length ?? 0,
    sortType: 'number',
    cell: (it) => (
      <span title={it.fields.models?.join('，')}><ModelCountCell models={it.fields.models} variant="count" /></span>
    ),
  }),
  numberCol('discount', '上游分组倍率', (it) => it.ext.discount, {
    width: 9,
    cell: (it) => (
      <>
        {it.ext.discount !== undefined ? formatNumber(it.ext.discount, 3) : DASH}
        {it.ext.discount_source === 'manual' && <Badge tone="warning" className="ml-1">手动</Badge>}
      </>
    ),
  }),
  numberCol('priority', '优先级', (it) => it.fields.priority, { width: 5, cell: (it) => it.fields.priority ?? DASH }),
  numberCol('weight', '权重', (it) => it.fields.weight, { width: 4.5, cell: (it) => it.fields.weight ?? DASH }),
  textCol('test_model', '测试模型', (it) => it.fields.test_model, { width: 10, cell: (it) => it.fields.test_model || DASH }),
];

interface StepPreviewProps {
  preview: UseMutationResult<BatchPreview, Error, BatchRequest>;
  tokenNames: Map<number, string>;
}

export function StepPreview({ preview, tokenNames }: StepPreviewProps) {
  const items = preview.data?.items ?? NO_ITEMS;
  const warnings = preview.data?.warnings ?? [];
  const byToken = Object.entries(preview.data?.unassigned_by_token ?? {}).filter(([, models]) => models.length > 0);
  const unassigned = preview.data?.unassigned ?? [];
  const duplicates = items.filter((i) => i.duplicate).length;

  return (
    <div className="grid gap-2">
      {preview.isPending && <div className="py-6 text-center text-muted-foreground">生成预览中…</div>}
      {preview.isError && <Banner tone="error">{errorText(preview.error)}</Banner>}
      {warnings.length > 0 && (
        <Banner tone="warning">
          {warnings.map((w) => <div key={w}>{w}</div>)}
        </Banner>
      )}
      {byToken.length > 0 ? (
        <Banner tone="warning">
          <div>以下模型不属于任何厂商，不会生成渠道：</div>
          {byToken.map(([tokenId, models]) => (
            <div key={tokenId}>{tokenNames.get(Number(tokenId)) ?? `令牌 ${tokenId}`}：{models.join('，')}</div>
          ))}
        </Banner>
      ) : (
        unassigned.length > 0 && (
          <Banner tone="warning">
            以下模型不属于任何厂商，不会生成渠道：{unassigned.join('，')}
          </Banner>
        )
      )}
      {duplicates > 0 && (
        <Banner tone="error">
          有 {duplicates} 个渠道名称重复（标红），请返回修改后再加入草稿。
        </Banner>
      )}
      {preview.isSuccess && (
        <DataTable<BatchPreviewItem>
          layoutId="wizard.preview"
          className="max-h-80"
          data={items}
          columns={PREVIEW_COLUMNS}
          getRowId={previewRowId}
          status={PREVIEW_STATUS}
        />
      )}
    </div>
  );
}
