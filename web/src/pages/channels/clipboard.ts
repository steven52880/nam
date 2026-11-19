import type { ChannelChange, ChannelRow } from '@/api/types';
import { COPYABLE_COLUMNS } from './constants';
import { copyText, mergePatch, parseCell, toChange, type Patch } from './edits';

const EMPTY_PATCH: Patch = { fields: {}, ext: {} };

/** 可复制的列；选了模型时折扣列显示的是模型级折扣，不是渠道自己的折扣，不能复制也不能粘贴。 */
export function copyableColumns(model: string | null): string[] {
  return model === null ? COPYABLE_COLUMNS : COPYABLE_COLUMNS.filter((c) => c !== 'discount');
}

/** 复制和粘贴共用的列投影：选区列里可复制的列，保持选区顺序。 */
export function copyColumnsOf(columnIds: string[], model: string | null = null): string[] {
  const allowed = copyableColumns(model);
  return columnIds.filter((c) => allowed.includes(c));
}

/** 一行的复制文本：只含可复制的列，用制表符连接。 */
export function copyRowText(row: ChannelRow, columnIds: string[], model: string | null = null): string {
  return copyColumnsOf(columnIds, model)
    .map((c) => copyText(row, c))
    .join('\t');
}

export interface PastePlan {
  changes: ChannelChange[];
  /** 不可粘贴或内容无效的列 id（去重）。 */
  skipped: string[];
}

/**
 * 粘贴：第 i 个可复制列取第 i 段文本；只有一段文本时广播到所有可复制列。
 * 每行合并成一个修改；没有任何有效内容的行不产生修改。
 */
export function planPaste(rows: ChannelRow[], columnIds: string[], text: string, model: string | null = null): PastePlan {
  const cells = text.replace(/\r?\n$/, '').split('\t');
  const copyable = copyColumnsOf(columnIds, model);
  const skipped = columnIds.filter((c) => !copyable.includes(c)).filter((c, i, all) => all.indexOf(c) === i);
  const changes: ChannelChange[] = [];
  for (const row of rows) {
    let merged = EMPTY_PATCH;
    copyable.forEach((columnId, index) => {
      const value = cells.length === 1 ? cells[0] : cells[index];
      if (value === undefined) return;
      const parsed = parseCell(columnId, value, row);
      if ('error' in parsed) {
        if (!skipped.includes(columnId)) skipped.push(columnId);
        return;
      }
      merged = mergePatch(merged, parsed.patch);
    });
    if (Object.keys(merged.fields).length + Object.keys(merged.ext).length > 0) changes.push(toChange(row.id, merged));
  }
  return { changes, skipped };
}
