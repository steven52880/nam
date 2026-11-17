import { Plus } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useChannels, useLatestCacheTests, useVendors } from '@/api/queries';
import type { ChannelRow, StatsWindow } from '@/api/types';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { DataGrid, type GridRange } from '@/components/data-grid';
import { PageHeader } from '@/components/layout/PageHeader';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { ApplyDialog } from '@/components/entities/ApplyDialog';
import { BatchWizard } from './channels/wizard/BatchWizard';
import { ChannelToolbar } from './channels/ChannelToolbar';
import { buildChannelColumns, rowClass } from './channels/columns';
import { costRangeOf } from '@/components/cells/costColor';
import { CacheTestDialog } from './channels/CacheTestDialog';
import { COLUMN_FIELDS, type GroupMode } from './channels/constants';
import { DriftDialog } from './channels/DriftDialog';
import { BulkUpstreamSelect } from './channels/EditableCells';
import { distinct, EMPTY_FILTERS, filterChannels, groupKeyOf, rowKey, type ChannelFilters } from './channels/filter';
import { ModelDetail } from './channels/ModelDetail';
import { ModelEditorDialog } from './channels/ModelEditorDialog';
import { RowContextMenu, type MenuItem } from './channels/RowContextMenu';
import { SplitPane } from './channels/SplitPane';
import { TestChannelDialog } from './channels/TestChannelDialog';
import { useChannelActions } from './channels/useChannelActions';
import { buildStatColumns } from './channels/statColumns';
import { summarizeGroup, useChannelStats } from './channels/useChannelStats';

const NO_ROWS: ChannelRow[] = [];

export default function ChannelsPage() {
  const list = useChannels();
  const [params, setParams] = useSearchParams();
  const rows = list.data?.items ?? NO_ROWS;
  const draftCount = list.data?.draft_count ?? 0;
  const actions = useChannelActions(rows);
  const { setUpstream, setToken, setVendor } = actions;

  const [groupMode, setGroupMode] = useState<GroupMode>('upstream');
  const [filters, setFilters] = useState<ChannelFilters>(() => ({ ...EMPTY_FILTERS, search: params.get('search') ?? '' }));
  const [statsWindow, setStatsWindow] = useState<StatsWindow>('latest');
  const { stats, errorLogHint } = useChannelStats(statsWindow);
  const [focusId, setFocusId] = useState<string | null>(null);
  const [range, setRange] = useState<GridRange | null>(null);
  const [modelRow, setModelRow] = useState<ChannelRow | null>(null);
  const [driftRow, setDriftRow] = useState<ChannelRow | null>(null);
  const [testRow, setTestRow] = useState<ChannelRow | null>(null);
  const [cacheRow, setCacheRow] = useState<ChannelRow | null>(null);
  const [deleteRow, setDeleteRow] = useState<ChannelRow | null>(null);
  const [menu, setMenu] = useState<{ row: ChannelRow; x: number; y: number } | null>(null);
  const [applyOpen, setApplyOpen] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(false);
  const [wizardOpen, setWizardOpen] = useState(false);
  const [wizardMode, setWizardMode] = useState<'single' | 'batch'>('batch');
  const openWizard = (mode: 'single' | 'batch') => {
    setWizardMode(mode);
    setWizardOpen(true);
  };

  const batchParam = Number(params.get('batch')) || null;
  const tokenParam = Number(params.get('token')) || null;
  const wizardVisible = wizardOpen || batchParam !== null;
  const closeWizard = (open: boolean) => {
    setWizardOpen(open);
    if (!open && params.has('batch')) {
      const next = new URLSearchParams(params);
      next.delete('batch');
      next.delete('token');
      setParams(next, { replace: true });
    }
  };

  const visible = useMemo(() => filterChannels(rows, filters), [rows, filters]);
  const focused = useMemo(() => rows.find((r) => rowKey(r) === focusId) ?? null, [rows, focusId]);
  const selectedRows = useMemo(
    () => (range ? rows.filter((r) => range.rowIds.includes(rowKey(r))) : NO_ROWS),
    [rows, range],
  );

  const channelCostRange = useMemo(() => costRangeOf(visible.map((r) => r.channel_cost)), [visible]);
  const unitCostRange = useMemo(() => costRangeOf(visible.map((r) => r.unit_cost)), [visible]);
  const statColumns = useMemo(
    () =>
      buildStatColumns({
        stats,
        effectiveCostRange: costRangeOf(visible.map((r) => stats.get(r.id)?.effective_cost_price ?? null)),
        stdCostRange: costRangeOf(visible.map((r) => stats.get(r.id)?.std_cost_price ?? null)),
      }),
    [stats, visible],
  );
  const vendors = useVendors();
  const vendorGroups = useMemo(
    () => new Map((vendors.data ?? []).map((v) => [v.name, v.group_name])),
    [vendors.data],
  );
  const cacheTestList = useLatestCacheTests();
  const cacheTests = useMemo(
    () => new Map((cacheTestList.data?.items ?? []).map((t) => [t.channel_id, t])),
    [cacheTestList.data],
  );
  const columns = useMemo(
    () =>
      buildChannelColumns({
        onOpenModels: setModelRow,
        onOpenDrift: setDriftRow,
        onSetUpstream: (row, upstreamId) => setUpstream.mutate({ targets: [row], upstreamId }),
        onSetToken: setToken,
        onSetVendor: setVendor,
        vendorGroups,
        cacheTests,
        channelCostRange,
        unitCostRange,
        statColumns,
      }),
    [channelCostRange, unitCostRange, statColumns, vendorGroups, cacheTests, setUpstream, setToken, setVendor],
  );
  const rowById = useMemo(() => new Map(rows.map((r) => [rowKey(r), r])), [rows]);

  const isDirty = useCallback(
    (rowId: string, columnId: string) => {
      const row = rowById.get(rowId);
      const fields = COLUMN_FIELDS[columnId];
      return !!row && !!fields && fields.some((f) => row.dirty_fields.includes(f));
    },
    [rowById],
  );
  const cellClassName = useCallback((row: ChannelRow, columnId: string) => {
    const fields = COLUMN_FIELDS[columnId];
    return fields?.some((f) => row.drift_fields.includes(f)) ? 'bg-destructive/15' : undefined;
  }, []);

  const menuItems = (row: ChannelRow): MenuItem[] => [
    row.status === 1
      ? { label: '禁用', onSelect: () => actions.setStatus([row], 2) }
      : { label: '启用', onSelect: () => actions.setStatus([row], 1) },
    { label: '测试渠道', onSelect: () => setTestRow(row), disabled: row.sync_state === 'draft_create' },
    {
      label: '测缓存',
      onSelect: () => setCacheRow(row),
      disabled: row.id < 0 || row.upstream_id === null || row.token_id === null,
      title:
        row.id < 0
          ? '草稿新渠道尚未写入 NewAPI，无法测缓存'
          : row.upstream_id === null || row.token_id === null
            ? '该渠道不是由 nam 创建或接管的，无法直连上游测试'
            : undefined,
    },
    { label: '在 NewAPI 中打开', onSelect: () => actions.openInNewapi.mutate(row.id), disabled: row.id < 0 },
    { label: '复制为新渠道', onSelect: () => actions.copyAsNew.mutate(row) },
    { label: '查看漂移差异', onSelect: () => setDriftRow(row), disabled: row.sync_state !== 'drift' },
    { label: '删除', destructive: true, onSelect: () => setDeleteRow(row) },
  ];

  const toolbar = (
    <>
      <span className="text-sm text-muted-foreground">{visible.length} / {rows.length} 个渠道</span>
      {selectedRows.length > 0 && (
        <>
          <Button size="sm" variant="outline" onClick={() => actions.setStatus(selectedRows, 1)}>批量启用（{selectedRows.length}）</Button>
          <Button size="sm" variant="outline" onClick={() => actions.setStatus(selectedRows, 2)}>批量禁用（{selectedRows.length}）</Button>
          <BulkUpstreamSelect
            count={selectedRows.length}
            onPick={(upstreamId) => setUpstream.mutate({ targets: selectedRows, upstreamId })}
          />
        </>
      )}
    </>
  );

  return (
    <div className="flex h-full flex-col gap-2 p-4">
      <PageHeader
        title="渠道"
        description="编辑只进入草稿，点「应用变更」后才会写入 NewAPI。"
        actions={
          <>
            <Badge tone={draftCount > 0 ? 'warning' : 'neutral'}>草稿 {draftCount}</Badge>
            <Button variant="outline" disabled={draftCount === 0} onClick={() => setDiscardOpen(true)}>放弃草稿</Button>
            <Button variant="outline" disabled={draftCount === 0} onClick={() => setApplyOpen(true)}>应用变更</Button>
            <Button variant="outline" onClick={() => openWizard('single')}><Plus className="h-3.5 w-3.5" />新建渠道</Button>
            <Button onClick={() => openWizard('batch')}><Plus className="h-3.5 w-3.5" />批量创建</Button>
          </>
        }
      />
      <ChannelToolbar
        groupMode={groupMode}
        onGroupMode={setGroupMode}
        filters={filters}
        onFilters={setFilters}
        upstreams={distinct(rows, (r) => r.upstream_name)}
        vendors={distinct(rows, (r) => r.vendor)}
        tags={distinct(rows, (r) => r.tag)}
        statsWindow={statsWindow}
        onStatsWindow={setStatsWindow}
      />
      {list.isError && <div className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">{list.error.message}</div>}
      {errorLogHint && (
        <div className="rounded bg-warning/15 px-2 py-1 text-sm text-warning">
          最近 7 天没有任何错误日志，NewAPI 可能没开 <code>ERROR_LOG_ENABLED</code>，成功率会偏高
        </div>
      )}
      <SplitPane
        top={
          <DataGrid<ChannelRow>
            className="min-h-0 flex-1"
            data={visible}
            columns={columns}
            getRowId={rowKey}
            groupBy={groupKeyOf(groupMode)}
            groupSummary={(_key, groupRows) => summarizeGroup(groupRows, stats)}
            isDirty={isDirty}
            rowClassName={rowClass}
            cellClassName={cellClassName}
            toolbar={toolbar}
            onEdit={actions.edit}
            onCopy={actions.copy}
            onPaste={actions.paste}
            onFocusRowChange={setFocusId}
            onSelectionChange={setRange}
            onRowContextMenu={(row, event) => {
              event.preventDefault();
              setMenu({ row, x: event.clientX, y: event.clientY });
            }}
          />
        }
        bottom={<ModelDetail row={focused} statsWindow={statsWindow} />}
      />
      <RowContextMenu state={menu} items={menuItems} onClose={() => setMenu(null)} />
      <ModelEditorDialog row={modelRow} onClose={() => setModelRow(null)} onSave={actions.saveModels} />
      <DriftDialog row={driftRow} onClose={() => setDriftRow(null)} />
      <CacheTestDialog row={cacheRow} onClose={() => setCacheRow(null)} />
      <TestChannelDialog row={testRow} onClose={() => setTestRow(null)} />
      <ApplyDialog open={applyOpen} onOpenChange={setApplyOpen} />
      <BatchWizard
        open={wizardVisible}
        onOpenChange={closeWizard}
        mode={batchParam !== null && !wizardOpen ? 'batch' : wizardMode}
        presetUpstreamId={batchParam}
        presetTokenId={tokenParam}
      />
      <ConfirmDialog
        open={deleteRow !== null}
        onOpenChange={(open) => { if (!open) setDeleteRow(null); }}
        title="删除渠道"
        description={`将「${deleteRow?.name ?? ''}」的删除加入草稿，应用变更后才会真正删除。`}
        confirmLabel="加入草稿"
        destructive
        onConfirm={() => {
          if (deleteRow) actions.remove.mutate([deleteRow.id]);
          setDeleteRow(null);
        }}
      />
      <ConfirmDialog
        open={discardOpen}
        onOpenChange={setDiscardOpen}
        title="放弃全部草稿"
        description={`将丢弃 ${draftCount} 条未应用的草稿，此操作不能撤销。`}
        confirmLabel="放弃草稿"
        destructive
        onConfirm={() => {
          actions.discard.mutate();
          setDiscardOpen(false);
        }}
      />
    </div>
  );
}
