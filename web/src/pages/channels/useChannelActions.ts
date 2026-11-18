import { useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { issueText } from '@/api/errors';
import { invalidate } from '@/api/invalidate';
import type { ChannelChange, ChannelRow } from '@/api/types';
import type { GridRange } from '@/components/table';
import { useActionMutation } from '@/hooks/useActionMutation';
import { copyableColumns, copyColumnsOf, copyRowText, planPaste } from './clipboard';
import { parseCell, toChange } from './edits';
import { useDraftPatch } from './useChannels';

const COPY_LABELS: Record<string, string> = {
  status: '状态',
  discount: '上游分组倍率',
  priority: '优先级',
  weight: '权重',
  tag: '标签',
  models: '模型',
};

/** model：当前选中的模型；选了模型时折扣列显示的是模型级折扣，不能通过编辑、复制、粘贴改渠道折扣。 */
export function useChannelActions(rows: ChannelRow[], model: string | null) {
  const qc = useQueryClient();
  // 只依赖 mutate（引用稳定）；mutation 对象本身每次状态变化都是新的，依赖它会让列定义反复重建
  const { mutate: patchMutate } = useDraftPatch();
  const { mutate: pasteMutate } = useDraftPatch((count) => `已修改 ${count} 格`);
  const byId = useCallback((id: string) => rows.find((r) => String(r.id) === id), [rows]);

  const edit = useCallback(
    (rowId: string, columnId: string, value: string) => {
      if (model !== null && columnId === 'discount') return;
      const row = byId(rowId);
      const parsed = parseCell(columnId, value);
      if (!row) return;
      if ('error' in parsed) {
        toast.error(parsed.error);
        return;
      }
      patchMutate([toChange(row.id, parsed.patch)]);
    },
    [byId, patchMutate, model],
  );

  const saveModels = useCallback(
    (row: ChannelRow, models: string[]) => patchMutate([toChange(row.id, { fields: { models }, ext: {} })]),
    [patchMutate],
  );

  const setStatus = useCallback(
    (targets: ChannelRow[], status: number) => {
      const changes = targets.filter((r) => r.status !== status).map((r) => toChange(r.id, { fields: { status }, ext: {} }));
      if (changes.length === 0) {
        toast.info('所选渠道已经是该状态');
        return;
      }
      pasteMutate(changes);
    },
    [pasteMutate],
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
      const allowed = copyColumnsOf(range.columnIds, model);
      const skipped = range.columnIds.length - allowed.length;
      if (allowed.length === 0) {
        toast.info(`所选列不能复制，可复制：${copyableColumns(model).map((c) => COPY_LABELS[c] ?? c).join('、')}`);
        return;
      }
      if (skipped > 0) toast.info(`已跳过 ${skipped} 个不能复制的列`);
      return copyRowText(row, range.columnIds, model);
    },
    [byId, model],
  );

  /** Ctrl+V：单行文本粘贴到选区内每一行（按可复制列依次对应），合并为一次 PATCH。 */
  const paste = useCallback(
    (range: GridRange, text: string) => {
      const targets = range.rowIds.map(byId).filter((r): r is ChannelRow => r !== undefined);
      const { changes, skipped } = planPaste(targets, range.columnIds, text, model);
      if (skipped.length > 0) toast.info(`有 ${skipped.length} 列因不可粘贴或内容无效被跳过`);
      if (changes.length === 0) {
        toast.info('没有可粘贴的内容');
        return;
      }
      pasteMutate(changes);
    },
    [byId, pasteMutate, model],
  );

  /** 改上游：先匹配令牌，匹配到就一起写入草稿；需要手动时只写上游并提示选令牌。 */
  const setUpstream = useActionMutation({
    mutationFn: async ({ targets, upstreamId }: { targets: ChannelRow[]; upstreamId: number }) => {
      const real = targets.filter((r) => r.id > 0 && r.sync_state !== 'draft_delete');
      if (real.length === 0) throw new Error('所选渠道还没有应用到 NewAPI，不能设置上游');
      const matched = await api.matchTokens({ upstream_id: upstreamId, channel_ids: real.map((r) => r.id) });
      const manual: string[] = [];
      const changes: ChannelChange[] = real.map((r) => {
        const item = matched.items.find((i) => i.channel_id === r.id);
        if (item && item.token_id !== null) return toChange(r.id, { fields: {}, ext: { upstream_id: upstreamId, token_id: item.token_id } });
        manual.push(`${r.name}：${item?.message || '无法自动匹配令牌'}`);
        return toChange(r.id, { fields: {}, ext: { upstream_id: upstreamId } });
      });
      const res = await api.patchDrafts(changes);
      return { res, manual, count: changes.length };
    },
    success: ({ count }) => `已修改 ${count} 个渠道的上游`,
    invalidate: invalidate.channels,
    onSuccess: ({ res, manual }) => {
      if (manual.length > 0) toast.warning(`请手动为以下渠道选择令牌：${manual.join('；')}`);
      const issues = res.issues ?? [];
      if (issues.length > 0) toast.warning(issues.map(issueText).join('；'));
    },
    onError: () => {
      void invalidate.channels(qc);
      return false;
    },
  });

  const setToken = useCallback(
    (row: ChannelRow, tokenId: number) => patchMutate([toChange(row.id, { fields: {}, ext: { token_id: tokenId } })]),
    [patchMutate],
  );

  const setVendor = useCallback(
    (row: ChannelRow, vendor: string) => patchMutate([toChange(row.id, { fields: {}, ext: { vendor } })]),
    [patchMutate],
  );

  const copyAsNew = useActionMutation({
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
    success: '已将副本加入草稿',
    invalidate: invalidate.channels,
  });

  const remove = useActionMutation({
    mutationFn: (ids: number[]) => api.deleteChannels(ids),
    success: '已将删除加入草稿',
    invalidate: invalidate.channels,
  });

  const discard = useActionMutation({
    mutationFn: () => api.discardDrafts(),
    success: '已放弃全部草稿',
    invalidate: invalidate.channels,
  });

  const openInNewapi = useActionMutation({
    mutationFn: (id: number) => api.newapiLink(id),
    onSuccess: ({ url }) => {
      window.open(url, '_blank', 'noopener');
    },
  });

  return { edit, saveModels, setStatus, setUpstream, setToken, setVendor, copy, paste, copyAsNew, remove, discard, openInNewapi };
}
