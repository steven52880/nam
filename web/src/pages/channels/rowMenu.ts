import type { ChannelRow } from '@/api/types';
import type { MenuItem } from '@/components/ui/menu-items';

export interface RowMenuHandlers {
  setStatus: (rows: ChannelRow[], status: number) => void;
  openTest: (row: ChannelRow) => void;
  openCache: (row: ChannelRow) => void;
  openInNewapi: (id: number) => void;
  copyAsNew: (row: ChannelRow) => void;
  askDelete: (row: ChannelRow) => void;
}

/** 渠道行右键菜单。禁用原因用 title 保留。 */
export function buildRowMenu(row: ChannelRow, h: RowMenuHandlers): MenuItem[] {
  const cacheReason =
    row.id < 0
      ? '草稿新渠道尚未写入 NewAPI，无法测缓存'
      : row.upstream_id === null || row.token_id === null
        ? '该渠道不是由 nam 创建或接管的，无法直连上游测试'
        : undefined;
  return [
    row.status === 1
      ? { key: 'disable', label: '禁用', onSelect: () => h.setStatus([row], 2) }
      : { key: 'enable', label: '启用', onSelect: () => h.setStatus([row], 1) },
    { key: 'test', label: '测试渠道', onSelect: () => h.openTest(row), disabled: row.sync_state === 'draft_create' },
    {
      key: 'cache',
      label: '测缓存',
      onSelect: () => h.openCache(row),
      disabled: row.id < 0 || row.upstream_id === null || row.token_id === null,
      title: cacheReason,
    },
    { key: 'newapi', label: '在 NewAPI 中打开', onSelect: () => h.openInNewapi(row.id), disabled: row.id < 0 },
    { key: 'copy', label: '复制为新渠道', onSelect: () => h.copyAsNew(row) },
    { key: 'delete', label: '删除', destructive: true, onSelect: () => h.askDelete(row) },
  ];
}
