import { useCallback, useMemo } from 'react';
import { COLUMN_FIELDS } from './constants';
import { rowKey } from './filter';
import { buildRowMenu } from './rowMenu';
import { useChannelColumns } from './useChannelColumns';
import type { ChannelsPageState } from './useChannelsPage';
import { summarizeGroup, type ChannelViewRow } from './viewModel';

/**
 * 传给 DataTable 的列、回调。全部引用稳定（依赖只取 mutate / useCallback / setState），
 * 否则拖列宽时表体会整个重新渲染。
 */
export function useChannelTable(page: ChannelsPageState) {
  const { rows, actions, dialogs } = page;
  const columns = useChannelColumns(page);

  const rowById = useMemo(() => new Map(rows.map((r) => [rowKey(r), r])), [rows]);
  const hasStats = page.stats.size > 0;
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
        askDelete: dialogs.setDeleteRow,
      }),
    [setStatus, openInNewapi, copyAsNew, dialogs.setTestRow, dialogs.setCacheRow, dialogs.setDeleteRow],
  );

  return { columns, groupSummary, isDirty, rowMenu };
}
