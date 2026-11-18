import type { CacheTest, ChannelRow } from '@/api/types';
import { BalanceCell } from '@/components/cells/BalanceCell';
import { CostCell } from '@/components/cells/CostCell';
import { Dash } from '@/components/cells/Dash';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { PercentCell } from '@/components/cells/PercentCell';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { numberCol, textCol, type TableColumn } from '@/components/table';
import { Badge } from '@/components/ui/misc';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';
import type { BalanceWidths } from '@/lib/balance';
import { formatDiscount, formatNumber, formatTime } from '@/lib/format';
import { CHANNEL_PRICE_HINT } from '@/lib/usdRate';
import { CHANNEL_STATUS, labelOf, SYNC_STATE } from '@/lib/labels';
import { cn } from '@/lib/utils';
import { TokenCell, UpstreamCell, VendorCell } from './EditableCells';
import { copyText, validateCell } from './edits';
import type { ChannelRanges, ChannelViewRow } from './viewModel';
import { buildStatColumns } from './statColumns';

/** 这几列的单元格自带下拉，双击 / Enter 时不进入行内文本编辑。 */
function noEditor(): void {
  return undefined;
}
const DISCOUNT_HINT = '上游对这个分组请求的实际扣费，是上游模型美元标价的多少倍（假设标价 = 原厂价）。例：0.35 表示按标价的 35% 扣费';
const CURRENCY_RATE_HINT = '在上游充值 1 美元额度需要多少人民币，单位 元/$。例：0.5 表示 0.5 元充 1 美元额度';

type Row = ChannelViewRow;
type Col = TableColumn<Row>;

interface ColumnOptions {
  /** 当前选中的模型；null = 渠道汇总。 */
  model: string | null;
  onOpenModels: (row: ChannelRow) => void;
  onOpenDrift: (row: ChannelRow) => void;
  onSetUpstream: (row: ChannelRow, upstreamId: number) => void;
  onSetToken: (row: ChannelRow, tokenId: number) => void;
  onSetVendor: (row: ChannelRow, vendor: string) => void;
  /** 厂商名 → 该厂商的托管分组名。 */
  vendorGroups: Map<string, string>;
  /** 渠道 id → 该渠道最近一次手动缓存率测试。 */
  cacheTests: Map<number, CacheTest>;
  /** 当前可见行的成本范围，用于颜色梯度。 */
  ranges: ChannelRanges;
  /** 余额列：当前可见行里美元、人民币文字的最大字符数，用于对齐 ≈。引用必须稳定。 */
  balanceWidths: BalanceWidths;
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

/** 列宽单位 rem（原像素 ÷ 16，取到 0.25）；列 id 上线后不改（保存的布局靠它）。 */
export function buildChannelColumns({
  model,
  onOpenModels,
  onOpenDrift,
  onSetUpstream,
  onSetToken,
  onSetVendor,
  vendorGroups,
  cacheTests,
  ranges,
  balanceWidths,
}: ColumnOptions): Col[] {
  const stat = buildStatColumns({ model, effectiveCostRange: ranges.effectiveCost, stdCostRange: ranges.stdCost });
  return [
    textCol<Row>('status', '状态', (r) => r.status, {
      width: 5.75,
      sortType: 'number',
      editable: true,
      toText: (r) => copyText(r, 'status'),
      validate: (v) => validateCell('status', v),
      cell: (r) => <StatusBadge dict={CHANNEL_STATUS} value={r.status} />,
    }),
    textCol<Row>('name', '名称', (r) => r.name, {
      width: 13.75,
      editable: true,
      validate: (v) => validateCell('name', v),
    }),
    textCol<Row>('upstream', '上游', (r) => r.upstream_name, {
      width: 8.25,
      editable: true,
      openEditor: noEditor,
      cell: (r) => <UpstreamCell row={r} onChange={onSetUpstream} />,
    }),
    textCol<Row>('token', '令牌', (r) => r.token_name, {
      width: 8.25,
      editable: true,
      openEditor: noEditor,
      cell: (r) => <TokenCell row={r} onChange={onSetToken} />,
    }),
    textCol<Row>('vendor', '厂商', (r) => r.vendor, {
      width: 7,
      editable: true,
      openEditor: noEditor,
      cell: (r) => <VendorCell row={r} onChange={onSetVendor} />,
    }),
    textCol<Row>('group', '分组', (r) => r.group, {
      width: 9.5,
      readOnlyReason: '托管分组随厂商变化，其他分组请在 NewAPI 中维护',
      headerTitle: '托管分组随厂商变化，其他分组请在 NewAPI 中维护',
      cell: (r) => <GroupBadges row={r} vendorGroups={vendorGroups} />,
    }),
    textCol<Row>('models', '模型', (r) => r.models.length, {
      width: 7,
      sortType: 'number',
      editable: true,
      openEditor: onOpenModels,
      toText: (r) => copyText(r, 'models'),
      validate: (v) => validateCell('models', v),
      cell: (r) => <ModelCountCell models={r.models} />,
    }),
    numberCol<Row>('discount', '上游分组倍率', (r) => r.view.discount, {
      width: 7.5,
      headerTitle: DISCOUNT_HINT,
      // 选了模型时显示的是模型级生效倍率，和渠道倍率不是同一个值，不允许在这里改
      editable: model === null,
      readOnlyReason: model === null ? undefined : '选了模型时这里显示该模型的生效倍率；改渠道倍率请先清除模型，改模型级倍率请在下方模型明细里改',
      toText: (r) => formatDiscount(r.discount),
      validate: (v) => validateCell('discount', v),
      cell: (r) => {
        if (r.view.discount === null) return <Dash />;
        // 选了模型：显示模型级生效倍率，来源徽标说的是渠道倍率，不再显示
        return (
          <span className="inline-flex items-center justify-end gap-1">
            {model === null && (
              <Badge tone={r.discount_source === 'manual' ? 'warning' : 'neutral'}>
                {r.discount_source === 'manual' ? '手动' : '自动'}
              </Badge>
            )}
            {formatDiscount(r.view.discount)}
          </span>
        );
      },
    }),
    numberCol<Row>('currency_rate', '上游货币汇率（元/$）', (r) => r.currency_rate, {
      width: 9,
      headerTitle: CURRENCY_RATE_HINT,
      readOnlyReason: '上游货币汇率在上游详情中修改',
      cell: (r) => formatNumber(r.currency_rate),
    }),
    numberCol<Row>('channel_cost', '上游渠道售价', (r) => r.view.channelCost, {
      width: 7.5,
      headerTitle: CHANNEL_PRICE_HINT,
      readOnlyReason: `${CHANNEL_PRICE_HINT}（不能直接编辑）`,
      cell: (r) => <CostCell percent value={r.view.channelCost} range={ranges.channelCost} />,
    }),
    numberCol<Row>('unit_cost', '单次成本（元）', (r) => r.view.unitCost, {
      width: 7.5,
      readOnlyReason: '单次成本由价格与倍率计算，不能编辑',
      cell: (r) => (
        <CostCell
          value={r.view.unitCost}
          range={ranges.unitCost}
          digits={4}
          significant
          hint={`按 ${r.view.unitCostModel || '—'} 的标准请求计算（1000 输入，其中 200 缓存，500 输出）`}
        />
      ),
    }),
    numberCol<Row>('balance', '余额', (r) => r.balance_usd, {
      width: 9.5,
      readOnlyReason: '余额来自上游同步，不能编辑',
      cell: (r) => <BalanceCell usd={r.balance_usd} cny={r.balance_cny} widths={balanceWidths} />,
    }),
    numberCol<Row>('priority', '优先级', (r) => r.priority, {
      width: 5,
      editable: true,
      validate: (v) => validateCell('priority', v),
    }),
    numberCol<Row>('weight', '权重', (r) => r.weight, {
      width: 4.5,
      editable: true,
      validate: (v) => validateCell('weight', v),
    }),
    textCol<Row>('tag', '标签', (r) => r.tag, {
      width: 6.25,
      editable: true,
      validate: (v) => validateCell('tag', v),
    }),
    ...stat.main,
    numberCol<Row>('manual_cache', '手动缓存率', (r) => cacheTests.get(r.id)?.rate ?? null, {
      width: 6.25,
      readOnlyReason: '手动缓存率来自「测缓存」，不能编辑',
      cell: (r) => <ManualCacheCell test={cacheTests.get(r.id)} />,
    }),
    ...stat.extra,
    textCol<Row>('sync_state', '同步状态', (r) => labelOf(SYNC_STATE, r.sync_state).label, {
      width: 6.25,
      readOnlyReason: '同步状态由系统维护，不能编辑',
      cell: (r) => <SyncBadge row={r} onOpenDrift={onOpenDrift} />,
    }),
  ];
}

export function rowClass(row: ChannelRow): string | undefined {
  return cn(row.status !== 1 && 'opacity-50', row.sync_state === 'draft_delete' && 'line-through');
}
