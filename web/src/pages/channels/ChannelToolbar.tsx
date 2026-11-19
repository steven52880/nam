import { useRef, useState } from 'react';
import type { StatsWindow } from '@/api/types';
import { FilterBar } from '@/components/filters/FilterBar';
import { FilterSelect } from '@/components/filters/FilterSelect';
import { ModelCombobox, type ModelOption } from '@/components/pickers/ModelCombobox';
import { SearchInput } from '@/components/filters/SearchInput';
import { Popover, PopoverAnchor } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { GROUP_MODE_OPTIONS, type GroupMode } from './constants';
import { CustomWindowContent } from './CustomWindowContent';
import { EMPTY_FILTERS, isFiltering, type ChannelFilters } from './filter';
import { optionsOf, CHANNEL_STATUS, STATS_WINDOW_PRESETS, SYNC_STATE, statsWindowLabel } from '@/lib/labels';
import { PRESET_WINDOWS, isPresetWindow } from '@/lib/statsWindow';

const STATUS_FILTER_OPTIONS = optionsOf(CHANNEL_STATUS);
const SYNC_FILTER_OPTIONS = optionsOf(SYNC_STATE, ['synced', 'draft', 'error']);
const STATS_WINDOW_OPTIONS = optionsOf(STATS_WINDOW_PRESETS, PRESET_WINDOWS);
/** 下拉里"自定义…"这一项的值，不会作为统计时长传出去。 */
const CUSTOM_ITEM = '__custom';

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
  /** 选中的模型；null = 渠道汇总。 */
  model: string | null;
  onModel: (model: string | null) => void;
  /** 全部渠道（未筛选）里的模型及其渠道数。 */
  modelOptions: ModelOption[];
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
  model,
  onModel,
  modelOptions,
}: ChannelToolbarProps) {
  const set = (key: keyof ChannelFilters) => (value: string) => onFilters({ ...filters, [key]: value });
  const pair = (items: string[]): Array<[string, string]> => items.map((i) => [i, i]);
  const [customOpen, setCustomOpen] = useState(false);
  const pickingCustom = useRef(false);
  const onWindowChange = (v: string) => {
    if (v !== CUSTOM_ITEM) {
      onStatsWindow(v as StatsWindow);
      return;
    }
    // 等下拉关闭、焦点处理完再开浮层，否则浮层会立刻被焦点变化关掉
    pickingCustom.current = true;
    setTimeout(() => setCustomOpen(true), 0);
  };
  return (
    <FilterBar
      clearable={isFiltering(filters)}
      onClear={() => onFilters(EMPTY_FILTERS)}
      trailing={
        <span className="ml-auto flex items-center gap-1 text-sm text-muted-foreground">
          统计时长
          <Popover open={customOpen} onOpenChange={setCustomOpen}>
            <PopoverAnchor asChild>
              <span>
                <Select value={statsWindow} onValueChange={onWindowChange}>
                  <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
                  <SelectContent
                    onCloseAutoFocus={(e) => {
                      if (!pickingCustom.current) return;
                      pickingCustom.current = false;
                      e.preventDefault();
                    }}
                  >
                    {STATS_WINDOW_OPTIONS.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}
                    {!isPresetWindow(statsWindow) && (
                      <SelectItem value={statsWindow}>{statsWindowLabel(statsWindow)}</SelectItem>
                    )}
                    <SelectItem value={CUSTOM_ITEM}>自定义…</SelectItem>
                  </SelectContent>
                </Select>
              </span>
            </PopoverAnchor>
            {customOpen && (
              <CustomWindowContent
                current={statsWindow}
                onSubmit={(w) => {
                  onStatsWindow(w);
                  setCustomOpen(false);
                }}
              />
            )}
          </Popover>
        </span>
      }
    >
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
      <SearchInput
        className="w-52"
        placeholder="搜索名称、令牌或模型"
        value={filters.search}
        onChange={(search) => onFilters({ ...filters, search })}
      />
      <ModelCombobox variant="toolbar" className="w-56" value={model} onChange={onModel} options={modelOptions} />
    </FilterBar>
  );
}
