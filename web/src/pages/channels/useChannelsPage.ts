import { useCallback, useMemo, useState } from 'react';
import { useChannels } from '@/api/queries';
import type { ChannelRow, StatsWindow } from '@/api/types';
import type { GridRange } from '@/components/table';
import { useLayoutSlot } from '@/lib/layout/LayoutProvider';
import { useUrlState } from '@/lib/url-state';
import type { GroupMode } from './constants';
import { EMPTY_FILTERS, rowKey, type ChannelFilters } from './filter';
import { CHANNELS_PAGE_SLOT } from './pageSlot';
import { useChannelActions } from './useChannelActions';
import { useChannelStats } from './useChannelStats';
import { buildChannelView } from './viewModel';

const NO_ROWS: ChannelRow[] = [];

/** 渠道页的全部页面状态：网址参数、布局槽位、筛选、统计时长、各弹窗开关、选区。 */
export function useChannelsPage() {
  const list = useChannels();
  const url = useUrlState();
  const rows = list.data?.items ?? NO_ROWS;
  const actions = useChannelActions(rows);

  // 布局槽位 page:channels：分组方式、下栏占比
  const [slot, setSlot] = useLayoutSlot(CHANNELS_PAGE_SLOT);
  const setGroupMode = useCallback((groupMode: GroupMode) => setSlot({ ...slot, groupMode }), [slot, setSlot]);
  const setSplitRatio = useCallback((splitRatio: number) => setSlot({ ...slot, splitRatio }), [slot, setSlot]);

  const [filters, setFilters] = useState<ChannelFilters>(() => ({ ...EMPTY_FILTERS, search: url.get('search') ?? '' }));
  const [statsWindow, setStatsWindow] = useState<StatsWindow>('latest');
  const { stats, errorLogHint } = useChannelStats(statsWindow);

  const view = useMemo(() => buildChannelView({ rows, stats, filters }), [rows, stats, filters]);

  // 选区与焦点
  const [focusId, setFocusId] = useState<string | null>(null);
  const [range, setRange] = useState<GridRange | null>(null);
  const focused = useMemo(() => rows.find((r) => rowKey(r) === focusId) ?? null, [rows, focusId]);
  const selectedRows = useMemo(
    () => (range ? rows.filter((r) => range.rowIds.includes(rowKey(r))) : NO_ROWS),
    [rows, range],
  );

  // 弹窗
  const [modelRow, setModelRow] = useState<ChannelRow | null>(null);
  const [driftRow, setDriftRow] = useState<ChannelRow | null>(null);
  const [testRow, setTestRow] = useState<ChannelRow | null>(null);
  const [cacheRow, setCacheRow] = useState<ChannelRow | null>(null);
  const [deleteRow, setDeleteRow] = useState<ChannelRow | null>(null);
  const [applyOpen, setApplyOpen] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(false);

  // 向导：网址参数 batch / token 可以直接打开
  const [wizardOpen, setWizardOpen] = useState(false);
  const [wizardMode, setWizardMode] = useState<'single' | 'batch'>('batch');
  const batchParam = Number(url.get('batch')) || null;
  const tokenParam = Number(url.get('token')) || null;
  const openWizard = (mode: 'single' | 'batch') => {
    setWizardMode(mode);
    setWizardOpen(true);
  };
  const closeWizard = (open: boolean) => {
    setWizardOpen(open);
    if (!open && url.get('batch') !== null) url.patch({ batch: null, token: null });
  };

  return {
    list,
    rows,
    draftCount: list.data?.draft_count ?? 0,
    actions,
    groupMode: slot.groupMode,
    setGroupMode,
    splitRatio: slot.splitRatio,
    setSplitRatio,
    filters,
    setFilters,
    statsWindow,
    setStatsWindow,
    stats,
    errorLogHint,
    view,
    focused,
    selectedRows,
    setFocusId,
    setRange,
    dialogs: {
      modelRow, setModelRow,
      driftRow, setDriftRow,
      testRow, setTestRow,
      cacheRow, setCacheRow,
      deleteRow, setDeleteRow,
      applyOpen, setApplyOpen,
      discardOpen, setDiscardOpen,
    },
    wizard: {
      visible: wizardOpen || batchParam !== null,
      mode: batchParam !== null && !wizardOpen ? ('batch' as const) : wizardMode,
      presetUpstreamId: batchParam,
      presetTokenId: tokenParam,
      open: openWizard,
      onOpenChange: closeWizard,
    },
  };
}

export type ChannelsPageState = ReturnType<typeof useChannelsPage>;
