import { useQuery } from '@tanstack/react-query';
import type { KeyboardEvent } from 'react';
import { api } from '@/api/endpoints';
import type { ChannelRow } from '@/api/types';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const DASH = '—';

/** 单元格内的下拉：阻止键盘事件冒泡到表格，避免触发表格的方向键 / Enter 处理。 */
function stopKeys(e: KeyboardEvent<HTMLElement>) {
  e.stopPropagation();
}

function locked(row: ChannelRow): boolean {
  return row.sync_state === 'draft_delete';
}

interface UpstreamCellProps {
  row: ChannelRow;
  onChange: (row: ChannelRow, upstreamId: number) => void;
}

export function UpstreamCell({ row, onChange }: UpstreamCellProps) {
  const upstreams = useQuery({ queryKey: ['upstreams'], queryFn: api.upstreams });
  return (
    <div onKeyDown={stopKeys}>
      <Select
        value={row.upstream_id === null ? '' : String(row.upstream_id)}
        disabled={locked(row)}
        onValueChange={(v) => onChange(row, Number(v))}
      >
        <SelectTrigger className="h-6 border-transparent bg-transparent px-1">
          <SelectValue placeholder={DASH}>{row.upstream_name || DASH}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {(upstreams.data ?? []).map((u) => <SelectItem key={u.id} value={String(u.id)}>{u.name}</SelectItem>)}
        </SelectContent>
      </Select>
    </div>
  );
}

interface TokenCellProps {
  row: ChannelRow;
  onChange: (row: ChannelRow, tokenId: number) => void;
}

export function TokenCell({ row, onChange }: TokenCellProps) {
  const upstreamId = row.upstream_id;
  const tokens = useQuery({
    queryKey: ['upstream-tokens', upstreamId],
    queryFn: () => api.upstreamTokens(upstreamId ?? 0),
    enabled: upstreamId !== null,
  });
  return (
    <div onKeyDown={stopKeys}>
      <Select
        value={row.token_id === null ? '' : String(row.token_id)}
        disabled={locked(row) || upstreamId === null}
        onValueChange={(v) => onChange(row, Number(v))}
      >
        <SelectTrigger className="h-6 border-transparent bg-transparent px-1" title={upstreamId === null ? '请先选择上游' : undefined}>
          <SelectValue placeholder={upstreamId === null ? DASH : '选择令牌'}>{row.token_name || (upstreamId === null ? DASH : '选择令牌')}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {(tokens.data ?? []).map((t) => (
            <SelectItem key={t.id} value={String(t.id)}>{t.name}{t.manual ? '（手动）' : ''}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

interface VendorCellProps {
  row: ChannelRow;
  onChange: (row: ChannelRow, vendor: string) => void;
}

export function VendorCell({ row, onChange }: VendorCellProps) {
  const vendors = useQuery({ queryKey: ['vendors'], queryFn: api.vendors });
  return (
    <div onKeyDown={stopKeys} title={row.vendor ? `NewAPI 分组：${row.group}` : undefined}>
      <Select value={row.vendor} disabled={locked(row)} onValueChange={(v) => onChange(row, v)}>
        <SelectTrigger className="h-6 border-transparent bg-transparent px-1">
          <SelectValue placeholder={DASH}>{row.vendor || DASH}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {(vendors.data ?? []).map((v) => <SelectItem key={v.id} value={v.name}>{v.name}</SelectItem>)}
          {vendors.isSuccess && (vendors.data ?? []).length === 0 && (
            <div className="px-2 py-1.5 text-sm text-muted-foreground">还没有厂商，请先到「设置 → 厂商分组」添加</div>
          )}
        </SelectContent>
      </Select>
    </div>
  );
}

/** 工具栏：对选中的多行批量设置上游。 */
export function BulkUpstreamSelect({ count, onPick }: { count: number; onPick: (upstreamId: number) => void }) {
  const upstreams = useQuery({ queryKey: ['upstreams'], queryFn: api.upstreams });
  return (
    <Select value="" onValueChange={(v) => onPick(Number(v))}>
      <SelectTrigger className="w-40"><SelectValue placeholder={`设置上游（${count}）`} /></SelectTrigger>
      <SelectContent>
        {(upstreams.data ?? []).map((u) => <SelectItem key={u.id} value={String(u.id)}>{u.name}</SelectItem>)}
      </SelectContent>
    </Select>
  );
}
