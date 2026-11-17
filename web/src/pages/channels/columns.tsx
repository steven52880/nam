import type { CacheTest, ChannelRow } from '@/api/types';
import { BalanceCell } from '@/components/cells/BalanceCell';
import { CostCell } from '@/components/cells/CostCell';
import { type CostRange } from '@/components/cells/costColor';
import { Dash } from '@/components/cells/Dash';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { PercentCell } from '@/components/cells/PercentCell';
import { StatusBadge } from '@/components/cells/StatusBadge';
import type { GridColumn } from '@/components/data-grid';
import { Badge } from '@/components/ui/misc';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import { formatDiscount, formatNumber, formatTime } from '@/lib/format';
import { CHANNEL_STATUS, labelOf, SYNC_STATE } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { TokenCell, UpstreamCell, VendorCell } from './EditableCells';
import { copyText, validateCell } from './edits';

/** 这几列的单元格自带下拉，双击 / Enter 时不进入行内文本编辑。 */
function noEditor(): void {
  return undefined;
}
const COST_REASON = '上游渠道成本价 = 上游分组折扣 × 上游货币汇率，不能直接编辑';

interface ColumnOptions {
  onOpenModels: (row: ChannelRow) => void;
  onOpenDrift: (row: ChannelRow) => void;
  onSetUpstream: (row: ChannelRow, upstreamId: number) => void;
  onSetToken: (row: ChannelRow, tokenId: number) => void;
  onSetVendor: (row: ChannelRow, vendor: string) => void;
  /** 厂商名 → 该厂商的托管分组名。 */
  vendorGroups: Map<string, string>;
  /** 渠道 id → 该渠道最近一次手动缓存率测试。 */
  cacheTests: Map<number, CacheTest>;
  /** 当前可见行的上游渠道成本价范围，用于颜色梯度。 */
  channelCostRange: CostRange | null;
  /** 当前可见行的单次成本范围。 */
  unitCostRange: CostRange | null;
  /** 统计列（成功率、延迟、实测成本等）。 */
  statColumns: { main: GridColumn<ChannelRow>[]; extra: GridColumn<ChannelRow>[] };
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

function SyncBadge({ row, onOpenDrift }: { row: ChannelRow; onOpenDrift: (row: ChannelRow) => void }) {
  const badge = <StatusBadge dict={SYNC_STATE} value={row.sync_state} />;
  if (row.sync_state === 'drift') {
    return (
      <button type="button" className="cursor-pointer" onClick={() => onOpenDrift(row)}>
        {badge}
      </button>
    );
  }
  if (row.sync_state === 'error') {
    return (
      <Tooltip>
        <TooltipTrigger asChild><span>{badge}</span></TooltipTrigger>
        <TooltipContent>{row.draft_error || '应用失败'}</TooltipContent>
      </Tooltip>
    );
  }
  return badge;
}

function GroupBadges({ row, vendorGroups }: { row: ChannelRow; vendorGroups: Map<string, string> }) {
  const groups = row.group.split(',').map((g) => g.trim()).filter(Boolean);
  if (groups.length === 0) return <Dash />;
  const managed = vendorGroups.get(row.vendor);
  return (
    <span className="inline-flex flex-wrap items-center gap-1">
      {groups.map((g) => <Badge key={g} tone={g === managed ? 'accent' : 'neutral'}>{g}</Badge>)}
    </span>
  );
}

function ManualCacheCell({ test }: { test: CacheTest | undefined }) {
  if (!test) return <Dash />;
  if (test.status === 'running') return <span className="text-muted-foreground">测试中…</span>;
  if (test.status === 'fail') {
    return (
      <Tooltip>
        <TooltipTrigger asChild><span className="text-destructive">失败</span></TooltipTrigger>
        <TooltipContent>{test.error || '测试失败'}</TooltipContent>
      </Tooltip>
    );
  }
  if (test.rate === null) return <Dash />;
  return (
    <Tooltip>
      <TooltipTrigger asChild><PercentCell value={test.rate} /></TooltipTrigger>
      <TooltipContent>{test.model} · {formatTime(test.started_at)}</TooltipContent>
    </Tooltip>
  );
}

export function buildChannelColumns({
  onOpenModels,
  onOpenDrift,
  onSetUpstream,
  onSetToken,
  onSetVendor,
  vendorGroups,
  cacheTests,
  channelCostRange,
  unitCostRange,
  statColumns,
}: ColumnOptions): GridColumn<ChannelRow>[] {
  return [
    {
      id: 'status',
      header: '状态',
      width: 90,
      editable: true,
      accessor: (r) => r.status,
      toText: (r) => copyText(r, 'status'),
      validate: (v) => validateCell('status', v),
      cell: (r) => <StatusBadge dict={CHANNEL_STATUS} value={r.status} />,
    },
    {
      id: 'name',
      header: '名称',
      width: 220,
      editable: true,
      accessor: (r) => r.name,
      validate: (v) => validateCell('name', v),
    },
    {
      id: 'upstream',
      header: '上游',
      width: 130,
      editable: true,
      openEditor: noEditor,
      accessor: (r) => r.upstream_name,
      cell: (r) => <UpstreamCell row={r} onChange={onSetUpstream} />,
    },
    {
      id: 'token',
      header: '令牌',
      width: 130,
      editable: true,
      openEditor: noEditor,
      accessor: (r) => r.token_name,
      cell: (r) => <TokenCell row={r} onChange={onSetToken} />,
    },
    {
      id: 'vendor',
      header: '厂商',
      width: 110,
      editable: true,
      openEditor: noEditor,
      accessor: (r) => r.vendor,
      cell: (r) => <VendorCell row={r} onChange={onSetVendor} />,
    },
    readOnly('group', '分组', (r) => r.group, '托管分组随厂商变化，其他分组请在 NewAPI 中维护', {
      width: 150,
      headerTitle: '托管分组随厂商变化，其他分组请在 NewAPI 中维护',
      cell: (r) => <GroupBadges row={r} vendorGroups={vendorGroups} />,
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
      cell: (r) => <ModelCountCell models={r.models} />,
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
      cell: (r) => <CostCell value={r.channel_cost} range={channelCostRange} />,
    }),
    readOnly('unit_cost', '单次成本（元）', (r) => r.unit_cost, '单次成本由价格与折扣计算，不能编辑', {
      width: 120,
      align: 'right',
      cell: (r) => (
        <CostCell
          value={r.unit_cost}
          range={unitCostRange}
          digits={4}
          significant
          hint={`按 ${r.unit_cost_model || '—'} 的标准请求计算（1000 输入，其中 200 缓存，500 输出）`}
        />
      ),
    }),
    readOnly('balance', '余额', (r) => r.balance_usd, '余额来自上游同步，不能编辑', {
      width: 150,
      align: 'right',
      cell: (r) => <BalanceCell usd={r.balance_usd} cny={r.balance_cny} />,
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
    ...statColumns.main,
    readOnly('manual_cache', '手动缓存率', (r) => cacheTests.get(r.id)?.rate ?? null, '手动缓存率来自「测缓存」，不能编辑', {
      width: 100,
      align: 'right',
      cell: (r) => <ManualCacheCell test={cacheTests.get(r.id)} />,
    }),
    ...statColumns.extra,
    readOnly('sync_state', '同步状态', (r) => labelOf(SYNC_STATE, r.sync_state).label, '同步状态由系统维护，不能编辑', {
      width: 100,
      cell: (r) => <SyncBadge row={r} onOpenDrift={onOpenDrift} />,
    }),
  ];
}

export function rowClass(row: ChannelRow): string | undefined {
  return cn(row.status !== 1 && 'opacity-50', row.sync_state === 'draft_delete' && 'line-through');
}
