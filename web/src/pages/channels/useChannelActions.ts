import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { ChannelChange, ChannelRow } from '@/api/types';
import type { GridRange } from '@/components/data-grid';
import { COPYABLE_COLUMNS } from './constants';
import { copyText, mergePatch, parseCell, toChange, type Patch } from './edits';
import { CHANNELS_KEY, useDraftPatch } from './useChannels';

const EMPTY_PATCH: Patch = { fields: {}, ext: {} };

export function useChannelActions(rows: ChannelRow[]) {
  const qc = useQueryClient();
  const patch = useDraftPatch();
  const pasteMutation = useDraftPatch((count) => `已修改 ${count} 格`);
  const byId = useCallback((id: string) => rows.find((r) => String(r.id) === id), [rows]);

  const edit = useCallback(
    (rowId: string, columnId: string, value: string) => {
      const row = byId(rowId);
      const parsed = parseCell(columnId, value);
      if (!row) return;
      if ('error' in parsed) {
        toast.error(parsed.error);
        return;
      }
      patch.mutate([toChange(row.id, parsed.patch)]);
    },
    [byId, patch],
  );

  const saveModels = useCallback(
    (row: ChannelRow, models: string[]) => patch.mutate([toChange(row.id, { fields: { models }, ext: {} })]),
    [patch],
  );

  const setStatus = useCallback(
    (targets: ChannelRow[], status: number) => {
      const changes = targets.filter((r) => r.status !== status).map((r) => toChange(r.id, { fields: { status }, ext: {} }));
      if (changes.length === 0) {
        toast.info('所选渠道已经是该状态');
        return;
      }
      pasteMutation.mutate(changes);
    },
    [pasteMutation],
  );

  /** Ctrl+C：只复制一行，仅限允许的列。 */
  const copy = useCallback(
    (range: GridRange): string | void => {
      if (range.rowIds.length !== 1) {
        toast.info('只能复制单行，请只选一行');
        return;
      }
      const row = byId(range.rowIds[0]);
      if (!row) return;
      const allowed = range.columnIds.filter((c) => COPYABLE_COLUMNS.includes(c));
      const skipped = range.columnIds.length - allowed.length;
      if (allowed.length === 0) {
        toast.info('所选列不能复制，可复制：状态、上游分组折扣、优先级、权重、标签、模型');
        return;
      }
      if (skipped > 0) toast.info(`已跳过 ${skipped} 个不能复制的列`);
      return allowed.map((c) => copyText(row, c)).join('\t');
    },
    [byId],
  );

  /** Ctrl+V：单行文本粘贴到选区内每一行（按列对应），合并为一次 PATCH。 */
  const paste = useCallback(
    (range: GridRange, text: string) => {
      const cells = text.replace(/\r?\n$/, '').split('\t');
      const changes: ChannelChange[] = [];
      const skipped: string[] = [];
      for (const rowId of range.rowIds) {
        const row = byId(rowId);
        if (!row) continue;
        let merged = EMPTY_PATCH;
        range.columnIds.forEach((columnId, index) => {
          if (!COPYABLE_COLUMNS.includes(columnId)) {
            if (!skipped.includes(columnId)) skipped.push(columnId);
            return;
          }
          const value = cells.length === 1 ? cells[0] : cells[index];
          if (value === undefined) return;
          const parsed = parseCell(columnId, value);
          if ('error' in parsed) {
            if (!skipped.includes(columnId)) skipped.push(columnId);
            return;
          }
          merged = mergePatch(merged, parsed.patch);
        });
        if (Object.keys(merged.fields).length + Object.keys(merged.ext).length > 0) changes.push(toChange(row.id, merged));
      }
      if (skipped.length > 0) toast.info(`有 ${skipped.length} 列因不可粘贴或内容无效被跳过`);
      if (changes.length === 0) {
        toast.info('没有可粘贴的内容');
        return;
      }
      pasteMutation.mutate(changes);
    },
    [byId, pasteMutation],
  );

  const copyAsNew = useMutation({
    mutationFn: (row: ChannelRow) =>
      api.batchCreate([
        {
          fields: {
            name: `${row.name}-copy`,
            base_url: row.base_url,
            models: row.models,
            group: row.group,
            priority: row.priority,
            weight: row.weight,
            tag: row.tag,
            status: row.status,
            test_model: row.test_model,
            type: row.type,
          },
          ext: {
            upstream_id: row.upstream_id ?? undefined,
            token_id: row.token_id ?? undefined,
            vendor: row.vendor,
            discount: row.discount,
            discount_source: row.discount_source,
          },
        },
      ]),
    onSuccess: () => {
      toast.success('已将副本加入草稿');
      void qc.invalidateQueries({ queryKey: CHANNELS_KEY });
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  const remove = useMutation({
    mutationFn: (ids: number[]) => api.deleteChannels(ids),
    onSuccess: () => {
      toast.success('已将删除加入草稿');
      void qc.invalidateQueries({ queryKey: CHANNELS_KEY });
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  const discard = useMutation({
    mutationFn: () => api.discardDrafts(),
    onSuccess: () => {
      toast.success('已放弃全部草稿');
      void qc.invalidateQueries({ queryKey: CHANNELS_KEY });
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  const openInNewapi = useMutation({
    mutationFn: (id: number) => api.newapiLink(id),
    onSuccess: ({ url }) => window.open(url, '_blank', 'noopener'),
    onError: (e: Error) => toast.error(errorText(e)),
  });

  return { edit, saveModels, setStatus, copy, paste, copyAsNew, remove, discard, openInNewapi };
}
