import type { ChannelRow } from '@/api/types';
import type { GridColumn } from '@/components/data-grid';
import { Badge } from '@/components/ui/misc';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatNumber } from '@/lib/format';
import { cn } from '@/lib/utils';
import { STATUS_LABELS, SYNC_LABELS } from './constants';
import { copyText, formatDiscount, validateCell } from './edits';

const DASH = '—';
const VENDOR_REASON = '厂商决定分组，在批量创建或设置中修改';
const COST_REASON = '上游渠道成本价 = 上游分组折扣 × 上游货币汇率，不能直接编辑';
const STATS_REASON = '统计列由系统计算，不能编辑';

interface ColumnOptions {
  onOpenModels: (row: ChannelRow) => void;
  onOpenDrift: (row: ChannelRow) => void;
}

function readOnly(
  id: string,
  header: string,
  accessor: GridColumn<ChannelRow>['accessor'],
  reason: string,
  extra: Partial<GridColumn<ChannelRow>> = {},
): GridColumn<ChannelRow> {
  return { id, header, accessor, readOnlyReason: reason, ...extra };
}

function statColumn(id: string, header: string, hidden = false): GridColumn<ChannelRow> {
  return readOnly(id, header, () => null, STATS_REASON, {
    width: 80,
    align: 'right',
    sortable: false,
    hidden,
    cell: () => <span className="text-muted-foreground">{DASH}</span>,
  });
}

function SyncBadge({ row, onOpenDrift }: { row: ChannelRow; onOpenDrift: (row: ChannelRow) => void }) {
  const label = SYNC_LABELS[row.sync_state];
  if (row.sync_state === 'drift') {
    return (
      <button type="button" className="cursor-pointer" onClick={() => onOpenDrift(row)}>
        <Badge tone="warning">{label}</Badge>
      </button>
    );
  }
  if (row.sync_state === 'error') {
    return (
      <Tooltip>
        <TooltipTrigger asChild><span><Badge tone="danger">{label}</Badge></span></TooltipTrigger>
        <TooltipContent>{row.draft_error || '应用失败'}</TooltipContent>
      </Tooltip>
    );
  }
  if (row.sync_state === 'synced') return <Badge tone="success">{label}</Badge>;
  return <Badge tone={row.sync_state === 'draft_delete' ? 'danger' : 'accent'}>{label}</Badge>;
}

export function buildChannelColumns({ onOpenModels, onOpenDrift }: ColumnOptions): GridColumn<ChannelRow>[] {
  return [
    {
      id: 'status',
      header: '状态',
      width: 90,
      editable: true,
      accessor: (r) => r.status,
      toText: (r) => copyText(r, 'status'),
      validate: (v) => validateCell('status', v),
      cell: (r) => (
        <Badge tone={r.status === 1 ? 'success' : r.status === 3 ? 'danger' : 'neutral'}>
          {STATUS_LABELS[r.status] ?? r.status}
        </Badge>
      ),
    },
    {
      id: 'name',
      header: '名称',
      width: 220,
      editable: true,
      accessor: (r) => r.name,
      validate: (v) => validateCell('name', v),
    },
    readOnly('upstream', '上游', (r) => r.upstream_name, '上游在批量创建时确定，不能修改', { width: 110 }),
    readOnly('token', '令牌', (r) => r.token_name, '令牌在批量创建时确定，不能修改', { width: 110 }),
    readOnly('vendor', '厂商', (r) => r.vendor, VENDOR_REASON, {
      width: 90,
      cell: (r) => (r.vendor ? <span title={`NewAPI 分组：${r.group}`}>{r.vendor}</span> : <span className="text-muted-foreground">{DASH}</span>),
    }),
    {
      id: 'models',
      header: '模型',
      width: 110,
      editable: true,
      openEditor: onOpenModels,
      accessor: (r) => r.models.length,
      toText: (r) => copyText(r, 'models'),
      validate: (v) => validateCell('models', v),
      cell: (r) => (
        <Tooltip>
          <TooltipTrigger asChild><span>{r.models.length} 个模型</span></TooltipTrigger>
          <TooltipContent className="max-h-60 overflow-y-auto">{r.models.join('，') || '无'}</TooltipContent>
        </Tooltip>
      ),
    },
    {
      id: 'discount',
      header: '上游分组折扣',
      width: 120,
      align: 'right',
      editable: true,
      accessor: (r) => r.discount,
      toText: (r) => formatDiscount(r.discount),
      validate: (v) => validateCell('discount', v),
      cell: (r) => (
        <span className="inline-flex items-center justify-end gap-1">
          <Badge tone={r.discount_source === 'manual' ? 'warning' : 'neutral'}>
            {r.discount_source === 'manual' ? '手动' : '自动'}
          </Badge>
          {formatDiscount(r.discount)}
        </span>
      ),
    },
    readOnly('currency_rate', '上游货币汇率', (r) => r.currency_rate, '上游货币汇率在上游详情中修改', {
      width: 110,
      align: 'right',
      cell: (r) => formatNumber(r.currency_rate),
    }),
    readOnly('channel_cost', '上游渠道成本价', (r) => r.channel_cost, COST_REASON, {
      width: 120,
      align: 'right',
      cell: (r) => formatNumber(r.channel_cost, 3),
    }),
    {
      id: 'priority',
      header: '优先级',
      width: 80,
      align: 'right',
      editable: true,
      accessor: (r) => r.priority,
      validate: (v) => validateCell('priority', v),
    },
    {
      id: 'weight',
      header: '权重',
      width: 70,
      align: 'right',
      editable: true,
      accessor: (r) => r.weight,
      validate: (v) => validateCell('weight', v),
    },
    {
      id: 'tag',
      header: '标签',
      width: 100,
      editable: true,
      accessor: (r) => r.tag,
      validate: (v) => validateCell('tag', v),
    },
    statColumn('success_rate', '成功率'),
    statColumn('first_token', '首字延迟'),
    statColumn('speed', '吐字速度'),
    statColumn('cache_hit', '缓存命中率', true),
    statColumn('cost_price', '成本价', true),
    statColumn('rating', '评级', true),
    statColumn('cost', '成本（元）', true),
    readOnly('sync_state', '同步状态', (r) => SYNC_LABELS[r.sync_state], '同步状态由系统维护，不能编辑', {
      width: 100,
      cell: (r) => <SyncBadge row={r} onOpenDrift={onOpenDrift} />,
    }),
  ];
}

export function rowClass(row: ChannelRow): string | undefined {
  return cn(row.status !== 1 && 'opacity-50', row.sync_state === 'draft_delete' && 'line-through');
}
