import type { ChannelChange, ChannelExtFields, ChannelFields, ChannelRow } from '@/api/types';
import { splitList } from '@/lib/list';
import { STATUS_LABELS } from './constants';

export interface Patch {
  fields: ChannelFields;
  ext: ChannelExtFields;
}

export type ParseResult = { patch: Patch } | { error: string };

function statusFromText(text: string): number | null {
  const t = text.trim();
  if (t === '启用' || t === '1') return 1;
  if (t === '禁用' || t === '手动禁用' || t === '2') return 2;
  return null;
}

/** 把单元格文本解析成补丁；失败返回中文错误。 */
export function parseCell(columnId: string, value: string): ParseResult {
  const text = value.trim();
  switch (columnId) {
    case 'name':
      if (!text) return { error: '名称不能为空' };
      return { patch: { fields: { name: text }, ext: {} } };
    case 'status': {
      const status = statusFromText(text);
      if (status === null) return { error: '状态只能填「启用」或「手动禁用」' };
      return { patch: { fields: { status }, ext: {} } };
    }
    case 'discount': {
      const discount = Number(text);
      if (text === '' || !Number.isFinite(discount) || discount <= 0) return { error: '上游分组折扣必须大于 0' };
      return { patch: { fields: {}, ext: { discount, discount_source: 'manual' } } };
    }
    case 'priority':
      if (!/^-?\d+$/.test(text)) return { error: '优先级必须是整数' };
      return { patch: { fields: { priority: Number(text) }, ext: {} } };
    case 'weight':
      if (!/^\d+$/.test(text)) return { error: '权重必须是不小于 0 的整数' };
      return { patch: { fields: { weight: Number(text) }, ext: {} } };
    case 'tag':
      return { patch: { fields: { tag: text }, ext: {} } };
    case 'models': {
      const models = splitList(text);
      if (models.length === 0) return { error: '模型不能为空' };
      return { patch: { fields: { models }, ext: {} } };
    }
    default:
      return { error: '该列不能编辑' };
  }
}

export function validateCell(columnId: string, value: string): string | null {
  const result = parseCell(columnId, value);
  return 'error' in result ? result.error : null;
}

export function mergePatch(base: Patch, extra: Patch): Patch {
  return { fields: { ...base.fields, ...extra.fields }, ext: { ...base.ext, ...extra.ext } };
}

export function toChange(channelId: number, patch: Patch): ChannelChange {
  return { channel_id: channelId, fields: patch.fields, ext: patch.ext };
}

/** 复制时使用的单元格文本。 */
export function copyText(row: ChannelRow, columnId: string): string {
  switch (columnId) {
    case 'status':
      return STATUS_LABELS[row.status] ?? String(row.status);
    case 'discount':
      return String(row.discount);
    case 'priority':
      return String(row.priority);
    case 'weight':
      return String(row.weight);
    case 'tag':
      return row.tag;
    case 'models':
      return row.models.join(',');
    default:
      return '';
  }
}

export function formatDiscount(value: number): string {
  return String(Number(value.toFixed(4)));
}
