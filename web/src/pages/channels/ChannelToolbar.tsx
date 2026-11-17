import type { StatsWindow } from '@/api/types';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { ALL_FILTER, GROUP_MODE_OPTIONS, STATUS_FILTER_OPTIONS, SYNC_FILTER_OPTIONS, type GroupMode } from './constants';
import { EMPTY_FILTERS, isFiltering, type ChannelFilters } from './filter';
import { STATS_WINDOW_OPTIONS } from './useChannelStats';

function FilterSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: Array<[string, string]>;
  onChange: (value: string) => void;
}) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL_FILTER}>全部{label}</SelectItem>
        {options.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
      </SelectContent>
    </Select>
  );
}

interface ChannelToolbarProps {
  groupMode: GroupMode;
  onGroupMode: (mode: GroupMode) => void;
  filters: ChannelFilters;
  onFilters: (next: ChannelFilters) => void;
  upstreams: string[];
  vendors: string[];
  tags: string[];
  statsWindow: StatsWindow;
  onStatsWindow: (window: StatsWindow) => void;
}

export function ChannelToolbar({
  groupMode,
  onGroupMode,
  filters,
  onFilters,
  upstreams,
  vendors,
  tags,
  statsWindow,
  onStatsWindow,
}: ChannelToolbarProps) {
  const set = (key: keyof ChannelFilters) => (value: string) => onFilters({ ...filters, [key]: value });
  const pair = (items: string[]): Array<[string, string]> => items.map((i) => [i, i]);
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Select value={groupMode} onValueChange={(v) => onGroupMode(v as GroupMode)}>
        <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
        <SelectContent>
          {GROUP_MODE_OPTIONS.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
        </SelectContent>
      </Select>
      <FilterSelect label="状态" value={filters.status} options={STATUS_FILTER_OPTIONS} onChange={set('status')} />
      <FilterSelect label="上游" value={filters.upstream} options={pair(upstreams)} onChange={set('upstream')} />
      <FilterSelect label="厂商" value={filters.vendor} options={pair(vendors)} onChange={set('vendor')} />
      <FilterSelect label="标签" value={filters.tag} options={pair(tags)} onChange={set('tag')} />
      <FilterSelect label="同步状态" value={filters.sync} options={SYNC_FILTER_OPTIONS} onChange={set('sync')} />
      <Input
        className="w-52"
        placeholder="搜索名称、令牌或模型"
        value={filters.search}
        onChange={(e) => onFilters({ ...filters, search: e.target.value })}
      />
      {isFiltering(filters) && <Button variant="link" onClick={() => onFilters(EMPTY_FILTERS)}>清除筛选</Button>}
      <span className="ml-auto flex items-center gap-1 text-sm text-muted-foreground">
        统计时长
        <Select value={statsWindow} onValueChange={(v) => onStatsWindow(v as StatsWindow)}>
          <SelectTrigger className="w-24"><SelectValue /></SelectTrigger>
          <SelectContent>
            {STATS_WINDOW_OPTIONS.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
          </SelectContent>
        </Select>
      </span>
    </div>
  );
}
