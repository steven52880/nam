import type { SlotSpec } from '@/lib/layout/store';
import { GROUP_MODE_OPTIONS, type GroupMode } from './constants';

/** 槽位 `page:channels`：分组方式、拖动后的组顺序、下栏占比。 */
export interface ChannelsPageSlot {
  groupMode: GroupMode;
  /** 上游 / 标签的组顺序（第五批启用，这批不读不写）。 */
  groupOrder: { upstream: string[]; tag: string[] };
  /** 下栏占容器高度的比例。 */
  splitRatio: number;
}

const MODES: string[] = GROUP_MODE_OPTIONS.map(([mode]) => mode);

function isStringArray(v: unknown): v is string[] {
  return Array.isArray(v) && v.every((x) => typeof x === 'string');
}

function parseSlot(data: unknown): ChannelsPageSlot | null {
  if (typeof data !== 'object' || data === null) return null;
  const d = data as Record<string, unknown>;
  const order = d.groupOrder as Record<string, unknown> | null | undefined;
  if (typeof d.groupMode !== 'string' || !MODES.includes(d.groupMode)) return null;
  if (typeof order !== 'object' || order === null) return null;
  if (!isStringArray(order.upstream) || !isStringArray(order.tag)) return null;
  if (typeof d.splitRatio !== 'number' || !Number.isFinite(d.splitRatio)) return null;
  return {
    groupMode: d.groupMode as GroupMode,
    groupOrder: { upstream: [...order.upstream], tag: [...order.tag] },
    splitRatio: d.splitRatio,
  };
}

/** 模块级常量：useSyncExternalStore 需要稳定的 spec 引用。 */
export const CHANNELS_PAGE_SLOT: SlotSpec<ChannelsPageSlot> = {
  id: 'page:channels',
  version: 1,
  // 这批保持现状按上游分组；第五批改成按厂商
  defaults: () => ({ groupMode: 'upstream', groupOrder: { upstream: [], tag: [] }, splitRatio: 0.3 }),
  parse: parseSlot,
};
