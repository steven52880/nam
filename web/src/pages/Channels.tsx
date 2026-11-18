import { Plus } from 'lucide-react';
import { PageHeader } from '@/components/layout/PageHeader';
import { SplitPane } from '@/components/layout/SplitPane';
import { DataTable } from '@/components/table';
import { Banner } from '@/components/ui/banner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { useLayoutEditable } from '@/lib/layout/LayoutProvider';
import { isBoolean, useLocalStorageState } from '@/lib/useLocalStorageState';
import { ChannelDialogs } from './channels/ChannelDialogs';
import { ChannelToolbar } from './channels/ChannelToolbar';
import { rowClass } from './channels/columns';
import { DEFAULT_SORT } from './channels/constants';
import { BulkUpstreamSelect } from './channels/EditableCells';
import { distinct, rowKey } from './channels/filter';
import { ModelDetail } from './channels/ModelDetail';
import { useChannelsPage } from './channels/useChannelsPage';
import { useChannelTable } from './channels/useChannelTable';
import type { ChannelViewRow } from './channels/viewModel';

const rowMenuHeader = (row: ChannelViewRow) => row.name;
const MODEL_DETAIL_COLLAPSED_KEY = 'nam.channels.modelDetailCollapsed';

export default function ChannelsPage() {
  const page = useChannelsPage();
  const { rows, actions, dialogs, wizard, view, groupMode, filters, grouping } = page;
  const table = useChannelTable(page);
  const editable = useLayoutEditable();
  // 下栏折叠状态只记在本机浏览器里，不进布局仓库
  const [detailCollapsed, setDetailCollapsed] = useLocalStorageState(MODEL_DETAIL_COLLAPSED_KEY, false, isBoolean);

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
        model={page.model}
        onModel={page.setModel}
        modelOptions={page.modelOptions}
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
        collapse={{ title: '模型明细', collapsed: detailCollapsed, onCollapsedChange: setDetailCollapsed }}
        top={
          <DataTable<ChannelViewRow>
            layoutId="channels.main"
            className="min-h-0 flex-1"
            data={view.rows}
            columns={table.columns}
            getRowId={rowKey}
            defaultSort={DEFAULT_SORT}
            selection="cells"
            groupBy={grouping.groupBy}
            groupLabel={grouping.groupLabel}
            sortGroups={grouping.sortGroups}
            onGroupReorder={grouping.onGroupReorder}
            groupSummary={table.groupSummary}
            groupResetKey={groupMode}
            groupsDefaultExpanded
            expandAll={filters.search.trim() !== ''}
            isDirty={table.isDirty}
            rowClassName={rowClass}
            cellClassName={table.cellClassName}
            toolbar={toolbar}
            onEdit={actions.edit}
            onCopy={actions.copy}
            onPaste={actions.paste}
            onFocusRowChange={page.setFocusId}
            onSelectionChange={page.setRange}
            rowMenu={table.rowMenu}
            rowMenuHeader={rowMenuHeader}
          />
        }
        bottom={<ModelDetail row={page.focused} statsWindow={page.statsWindow} />}
      />
      <ChannelDialogs page={page} />
    </div>
  );
}
