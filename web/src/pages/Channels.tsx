import { Plus } from 'lucide-react';
import { useCallback, useMemo } from 'react';
import { PageHeader } from '@/components/layout/PageHeader';
import { SplitPane } from '@/components/layout/SplitPane';
import { DataTable } from '@/components/table';
import { Banner } from '@/components/ui/banner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { useLayoutEditable } from '@/lib/layout/LayoutProvider';
import { ChannelDialogs } from './channels/ChannelDialogs';
import { ChannelToolbar } from './channels/ChannelToolbar';
import { rowClass } from './channels/columns';
import { COLUMN_FIELDS } from './channels/constants';
import { BulkUpstreamSelect } from './channels/EditableCells';
import { distinct, rowKey } from './channels/filter';
import { buildGroupLookups, groupKeyOf, groupLabelOf } from './channels/grouping';
import { ModelDetail } from './channels/ModelDetail';
import { buildRowMenu } from './channels/rowMenu';
import { useChannelColumns } from './channels/useChannelColumns';
import { useChannelsPage } from './channels/useChannelsPage';
import { summarizeGroup, type ChannelViewRow } from './channels/viewModel';

const rowMenuHeader = (row: ChannelViewRow) => row.name;

export default function ChannelsPage() {
  const page = useChannelsPage();
  const { rows, actions, dialogs, wizard, view, groupMode, filters } = page;
  const columns = useChannelColumns(page);
  const editable = useLayoutEditable();

  const rowById = useMemo(() => new Map(rows.map((r) => [rowKey(r), r])), [rows]);
  const lookups = useMemo(() => buildGroupLookups(rows), [rows]);
  const groupBy = useMemo(() => groupKeyOf(groupMode), [groupMode]);
  const groupLabel = useCallback((key: string) => groupLabelOf(groupMode, key, lookups), [groupMode, lookups]);
  const hasStats = page.stats.size > 0;
  // 传给表格的回调保持引用稳定，否则拖列宽时表体会整个重新渲染
  const groupSummary = useCallback(
    (_key: string, groupRows: ChannelViewRow[]) => summarizeGroup(groupRows, hasStats),
    [hasStats],
  );

  const isDirty = useCallback(
    (rowId: string, columnId: string) => {
      const row = rowById.get(rowId);
      const fields = COLUMN_FIELDS[columnId];
      return !!row && !!fields && fields.some((f) => row.dirty_fields.includes(f));
    },
    [rowById],
  );
  const cellClassName = useCallback((row: ChannelViewRow, columnId: string) => {
    const fields = COLUMN_FIELDS[columnId];
    return fields?.some((f) => row.drift_fields.includes(f)) ? 'bg-destructive/15' : undefined;
  }, []);
  // 只依赖引用稳定的函数（mutate、useCallback、setState），不依赖每次渲染都换新的 actions 对象
  const { setStatus } = actions;
  const openInNewapi = actions.openInNewapi.mutate;
  const copyAsNew = actions.copyAsNew.mutate;
  const rowMenu = useCallback(
    (row: ChannelViewRow) =>
      buildRowMenu(row, {
        setStatus,
        openTest: dialogs.setTestRow,
        openCache: dialogs.setCacheRow,
        openInNewapi: (id) => openInNewapi(id),
        copyAsNew: (r) => copyAsNew(r),
        openDrift: dialogs.setDriftRow,
        askDelete: dialogs.setDeleteRow,
      }),
    [setStatus, openInNewapi, copyAsNew, dialogs.setTestRow, dialogs.setCacheRow, dialogs.setDriftRow, dialogs.setDeleteRow],
  );

  const selected = page.selectedRows;
  const toolbar = (
    <>
      <span className="text-sm text-muted-foreground">{view.rows.length} / {rows.length} 个渠道</span>
      {selected.length > 0 && (
        <>
          <Button size="sm" variant="outline" onClick={() => actions.setStatus(selected, 1)}>批量启用（{selected.length}）</Button>
          <Button size="sm" variant="outline" onClick={() => actions.setStatus(selected, 2)}>批量禁用（{selected.length}）</Button>
          <BulkUpstreamSelect
            count={selected.length}
            onPick={(upstreamId) => actions.setUpstream.mutate({ targets: selected, upstreamId })}
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
            <Badge tone={page.draftCount > 0 ? 'warning' : 'neutral'}>草稿 {page.draftCount}</Badge>
            <Button variant="outline" disabled={page.draftCount === 0} onClick={() => dialogs.setDiscardOpen(true)}>放弃草稿</Button>
            <Button variant="outline" disabled={page.draftCount === 0} onClick={() => dialogs.setApplyOpen(true)}>应用变更</Button>
            <Button variant="outline" onClick={() => wizard.open('single')}><Plus className="h-3.5 w-3.5" />新建渠道</Button>
            <Button onClick={() => wizard.open('batch')}><Plus className="h-3.5 w-3.5" />批量创建</Button>
          </>
        }
      />
      <ChannelToolbar
        groupMode={groupMode}
        onGroupMode={page.setGroupMode}
        filters={filters}
        onFilters={page.setFilters}
        upstreams={distinct(rows, (r) => r.upstream_name)}
        vendors={distinct(rows, (r) => r.vendor)}
        tags={distinct(rows, (r) => r.tag)}
        statsWindow={page.statsWindow}
        onStatsWindow={page.setStatsWindow}
      />
      {page.list.isError && <Banner tone="error">{page.list.error.message}</Banner>}
      {page.errorLogHint && (
        <Banner tone="warning">
          最近 7 天没有任何错误日志，NewAPI 可能没开 <code>ERROR_LOG_ENABLED</code>，成功率会偏高
        </Banner>
      )}
      <SplitPane
        ratio={page.splitRatio}
        onRatioChange={page.setSplitRatio}
        disabled={!editable}
        top={
          <DataTable<ChannelViewRow>
            layoutId="channels.main"
            className="min-h-0 flex-1"
            data={view.rows}
            columns={columns}
            getRowId={rowKey}
            selection="cells"
            groupBy={groupBy}
            groupLabel={groupLabel}
            groupSummary={groupSummary}
            groupResetKey={groupMode}
            expandAll={filters.search.trim() !== ''}
            isDirty={isDirty}
            rowClassName={rowClass}
            cellClassName={cellClassName}
            toolbar={toolbar}
            onEdit={actions.edit}
            onCopy={actions.copy}
            onPaste={actions.paste}
            onFocusRowChange={page.setFocusId}
            onSelectionChange={page.setRange}
            rowMenu={rowMenu}
            rowMenuHeader={rowMenuHeader}
          />
        }
        bottom={<ModelDetail row={page.focused} statsWindow={page.statsWindow} />}
      />
      <ChannelDialogs page={page} />
    </div>
  );
}
