import type { Upstream } from '@/api/types';
import { upstreamColumns } from '@/components/entities/upstreamColumns';
import { customCol, pickColumns, textCol, type TableColumn } from '@/components/table';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { CRED_LABELS, splitTags } from './constants';
import { CredNote, isCredInvalid } from './credStatus';

export interface UpstreamPageColumnOptions {
  /** 正在同步的上游 id。 */
  syncingIds: ReadonlySet<number>;
  onSync: (id: number) => void;
  onReplaceCred: (upstream: Upstream) => void;
}

/** 上游页默认显示的列和顺序（含只在上游页出现的标签、凭证、错误、操作列）。 */
const PAGE_COLUMN_IDS = [
  'name', 'base_url', 'type', 'tags', 'currency_rate', 'cred', 'balance_usd', 'balance_cny',
  'token_count', 'channel_count', 'last_sync_at', 'error', 'actions',
];

/** 列宽单位 rem；列 id 上线后不改（保存的布局靠它）。 */
export function buildUpstreamPageColumns({ syncingIds, onSync, onReplaceCred }: UpstreamPageColumnOptions): TableColumn<Upstream>[] {
  const all: TableColumn<Upstream>[] = [
    ...upstreamColumns(),
    textCol<Upstream>('tags', '标签', (u) => u.tags, {
      width: 9,
      cell: (u) => (
        <span className="inline-flex gap-1">
          {splitTags(u.tags).map((t) => <Badge key={t} tone="accent">{t}</Badge>)}
        </span>
      ),
    }),
    textCol<Upstream>('cred', '凭证状态', (u) => (u.has_credential ? (CRED_LABELS[u.cred_type] ?? '已配置') : '未配置'), {
      width: 14,
      cell: (u) => (
        <span className="inline-flex items-center gap-1.5">
          {u.has_credential ? <Badge tone="success">{CRED_LABELS[u.cred_type] ?? '已配置'}</Badge> : <Badge tone="warning">未配置</Badge>}
          <CredNote u={u} />
        </span>
      ),
    }),
    textCol<Upstream>('error', '错误', (u) => u.last_sync_error, {
      width: 14,
      cell: (u) => (
        <span data-no-row-click className="inline-flex max-w-full items-center gap-1.5 text-destructive" title={u.last_sync_error}>
          <span className="truncate">{u.last_sync_error}</span>
          {isCredInvalid(u) && <Button size="sm" variant="outline" onClick={() => onReplaceCred(u)}>更换凭证</Button>}
        </span>
      ),
    }),
    customCol<Upstream>('actions', '操作', {
      width: 7.5,
      fixed: true,
      cell: (u) => {
        const syncing = syncingIds.has(u.id);
        return <Button size="sm" variant="outline" disabled={syncing} onClick={() => onSync(u.id)}>{syncing ? '同步中…' : '立即同步'}</Button>;
      },
    }),
  ];
  return pickColumns(all, PAGE_COLUMN_IDS);
}
