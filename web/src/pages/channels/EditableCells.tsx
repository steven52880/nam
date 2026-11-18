import { useVendors } from '@/api/queries';
import type { ChannelRow } from '@/api/types';
import { TokenPicker, UpstreamPicker, VendorPicker } from '@/components/pickers';
import { DASH } from '@/lib/format';

function locked(row: ChannelRow): boolean {
  return row.sync_state === 'draft_delete';
}

interface UpstreamCellProps {
  row: ChannelRow;
  onChange: (row: ChannelRow, upstreamId: number) => void;
}

export function UpstreamCell({ row, onChange }: UpstreamCellProps) {
  return (
    <UpstreamPicker
      variant="cell"
      value={row.upstream_id}
      disabled={locked(row)}
      placeholder={DASH}
      fallbackLabel={row.upstream_name || undefined}
      onChange={(id) => onChange(row, id)}
    />
  );
}

interface TokenCellProps {
  row: ChannelRow;
  onChange: (row: ChannelRow, tokenId: number) => void;
}

export function TokenCell({ row, onChange }: TokenCellProps) {
  const upstreamId = row.upstream_id;
  return (
    <TokenPicker
      variant="cell"
      upstreamId={upstreamId}
      value={row.token_id}
      disabled={locked(row) || upstreamId === null}
      title={upstreamId === null ? '请先选择上游' : undefined}
      placeholder={upstreamId === null ? DASH : '选择令牌'}
      fallbackLabel={row.token_name || undefined}
      onChange={(id) => onChange(row, id)}
    />
  );
}

interface VendorCellProps {
  row: ChannelRow;
  onChange: (row: ChannelRow, vendor: string) => void;
}

export function VendorCell({ row, onChange }: VendorCellProps) {
  const vendors = useVendors();
  const vendor = (vendors.data ?? []).find((v) => v.name === row.vendor);
  return (
    <div title={vendor ? `托管分组：${vendor.group_name}；改厂商会移除其他厂商的分组` : undefined}>
      <VendorPicker
        variant="cell"
        value={row.vendor}
        disabled={locked(row)}
        placeholder={DASH}
        fallbackLabel={row.vendor || undefined}
        onChange={(name) => onChange(row, name)}
      />
    </div>
  );
}

/** 工具栏：对选中的多行批量设置上游。 */
export function BulkUpstreamSelect({ count, onPick }: { count: number; onPick: (upstreamId: number) => void }) {
  return (
    <UpstreamPicker
      variant="toolbar"
      value={null}
      placeholder={`设置上游（${count}）`}
      className="w-40"
      onChange={onPick}
    />
  );
}
