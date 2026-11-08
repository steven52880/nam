import type { ChannelRow } from '@/api/types';
import { ALL_FILTER, type GroupMode } from './constants';

export interface ChannelFilters {
  status: string;
  upstream: string;
  vendor: string;
  tag: string;
  sync: string;
  search: string;
}

export const EMPTY_FILTERS: ChannelFilters = {
  status: ALL_FILTER,
  upstream: ALL_FILTER,
  vendor: ALL_FILTER,
  tag: ALL_FILTER,
  sync: ALL_FILTER,
  search: '',
};

export function isFiltering(f: ChannelFilters): boolean {
  return Object.values(f).some((v) => v !== ALL_FILTER && v !== '');
}

function syncMatches(row: ChannelRow, sync: string): boolean {
  if (sync === 'draft') return row.sync_state.startsWith('draft');
  return row.sync_state === sync;
}

export function filterChannels(rows: ChannelRow[], f: ChannelFilters): ChannelRow[] {
  const search = f.search.trim().toLowerCase();
  return rows.filter((r) => {
    if (f.status !== ALL_FILTER && String(r.status) !== f.status) return false;
    if (f.upstream !== ALL_FILTER && r.upstream_name !== f.upstream) return false;
    if (f.vendor !== ALL_FILTER && r.vendor !== f.vendor) return false;
    if (f.tag !== ALL_FILTER && r.tag !== f.tag) return false;
    if (f.sync !== ALL_FILTER && !syncMatches(r, f.sync)) return false;
    if (!search) return true;
    return (
      r.name.toLowerCase().includes(search) ||
      r.token_name.toLowerCase().includes(search) ||
      r.models.some((m) => m.toLowerCase().includes(search))
    );
  });
}

export function groupKeyOf(mode: GroupMode): ((row: ChannelRow) => string) | undefined {
  switch (mode) {
    case 'upstream':
      return (r) => r.upstream_name || '未托管';
    case 'vendor':
      return (r) => r.vendor || '无厂商';
    case 'tag':
      return (r) => r.tag || '无标签';
    case 'none':
      return undefined;
  }
}

export function distinct(rows: ChannelRow[], pick: (row: ChannelRow) => string): string[] {
  return [...new Set(rows.map(pick).filter(Boolean))].sort();
}

export function rowKey(row: ChannelRow): string {
  return String(row.id);
}
