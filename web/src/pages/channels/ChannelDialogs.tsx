import { ApplyDialog } from '@/components/entities/ApplyDialog';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { CacheTestDialog } from './CacheTestDialog';
import { ModelEditorDialog } from './ModelEditorDialog';
import { TestChannelDialog } from './TestChannelDialog';
import type { ChannelsPageState } from './useChannelsPage';
import { BatchWizard } from './wizard/BatchWizard';

/** 渠道页的全部弹窗，开关状态来自 useChannelsPage。 */
export function ChannelDialogs({ page }: { page: ChannelsPageState }) {
  const { dialogs: d, wizard, actions, draftCount } = page;
  return (
    <>
      <ModelEditorDialog row={d.modelRow} onClose={() => d.setModelRow(null)} onSave={actions.saveModels} />
      <CacheTestDialog row={d.cacheRow} onClose={() => d.setCacheRow(null)} />
      <TestChannelDialog row={d.testRow} onClose={() => d.setTestRow(null)} />
      <ApplyDialog open={d.applyOpen} onOpenChange={d.setApplyOpen} />
      <BatchWizard
        open={wizard.visible}
        onOpenChange={wizard.onOpenChange}
        mode={wizard.mode}
        presetUpstreamId={wizard.presetUpstreamId}
        presetTokenId={wizard.presetTokenId}
      />
      <ConfirmDialog
        open={d.deleteRow !== null}
        onOpenChange={(open) => { if (!open) d.setDeleteRow(null); }}
        title="删除渠道"
        description={`将「${d.deleteRow?.name ?? ''}」的删除加入草稿，应用变更后才会真正删除。`}
        confirmLabel="加入草稿"
        destructive
        onConfirm={() => {
          if (d.deleteRow) actions.remove.mutate([d.deleteRow.id]);
          d.setDeleteRow(null);
        }}
      />
      <ConfirmDialog
        open={d.discardOpen}
        onOpenChange={d.setDiscardOpen}
        title="放弃全部草稿"
        description={`将丢弃 ${draftCount} 条未应用的草稿，此操作不能撤销。`}
        confirmLabel="放弃草稿"
        destructive
        onConfirm={() => {
          actions.discard.mutate();
          d.setDiscardOpen(false);
        }}
      />
    </>
  );
}
