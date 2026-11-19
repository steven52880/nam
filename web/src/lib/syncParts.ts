import type { SyncPart, SyncPartKey, SyncParts } from '@/api/types';
import { formatAgo } from './format';

/** 上游「支持情况」四项：余额、令牌列表、分组倍率、逐模型价格。 */
export const SYNC_PART_ORDER: SyncPartKey[] = ['balance', 'tokens', 'groups', 'model_prices'];

export const SYNC_PART_LABELS: Record<SyncPartKey, string> = {
  balance: '余额',
  tokens: '令牌列表',
  groups: '分组倍率',
  model_prices: '逐模型价格',
};

/** 数量的单位：13 个分组。余额没有数量。 */
const COUNT_UNIT: Partial<Record<SyncPartKey, string>> = {
  tokens: '个令牌',
  groups: '个分组',
  model_prices: '条价格',
};

export type SyncPartView = {
  key: SyncPartKey;
  label: string;
  /** ok 绿勾；bad 红叉（fail / unsupported / empty）；none 灰色，还没同步。 */
  status: 'ok' | 'bad' | 'none';
  /** 一句话说明：失败原因、不支持、没有数据，或「还没同步」。成功时为空。 */
  reason: string;
  /** 数量文字，如「13 个分组」；没有数量为空。 */
  countText: string;
  /** 相对时间，如「10 分钟前」；没同步过为空。 */
  agoText: string;
};

const STATE_FALLBACK: Record<SyncPart['state'], string> = {
  ok: '',
  fail: '拉取失败',
  unsupported: '该上游类型不支持',
  empty: '没有数据',
};

/** 把一项同步结果整理成界面要显示的内容；part 为空表示还没同步过。 */
export function syncPartView(key: SyncPartKey, part: SyncPart | undefined, nowMs: number = Date.now()): SyncPartView {
  const label = SYNC_PART_LABELS[key];
  if (!part) return { key, label, status: 'none', reason: '还没同步', countText: '', agoText: '' };
  const unit = COUNT_UNIT[key];
  const countText = unit && part.count !== null && part.count !== undefined ? `${part.count} ${unit}` : '';
  const reason = part.state === 'ok' ? '' : part.message.trim() || STATE_FALLBACK[part.state];
  return {
    key,
    label,
    status: part.state === 'ok' ? 'ok' : 'bad',
    reason,
    countText,
    agoText: part.at ? formatAgo(part.at, nowMs) : '',
  };
}

/** 四项按固定顺序整理；parts 为空（老数据）时四项都是「还没同步」。 */
export function syncPartViews(parts: SyncParts | null | undefined, nowMs: number = Date.now()): SyncPartView[] {
  return SYNC_PART_ORDER.map((key) => syncPartView(key, parts?.[key], nowMs));
}
