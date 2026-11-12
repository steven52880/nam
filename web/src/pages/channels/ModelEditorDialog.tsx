import { useMemo } from 'react';
import { api } from '@/api/endpoints';
import type { ChannelRow } from '@/api/types';
import { ModelPickerDialog } from '@/components/model-picker';
import { useVendorMatches } from '@/components/model-picker/useModelGroups';

interface ModelEditorDialogProps {
  row: ChannelRow | null;
  onClose: () => void;
  onSave: (row: ChannelRow, models: string[]) => void;
}

const NO_MODELS: string[] = [];

/** 渠道模型编辑：外层只做取数与校验提示，勾选交给 ModelPickerDialog。 */
export function ModelEditorDialog({ row, onClose, onSave }: ModelEditorDialogProps) {
  const upstreamId = row?.upstream_id ?? null;
  const tokenId = row?.token_id ?? null;
  const vendor = row?.vendor ?? '';
  const linked = upstreamId !== null && tokenId !== null;

  const models = row?.models ?? NO_MODELS;

  // 令牌不再保存模型列表，候选只能现场向上游拉取：这里不放候选，
  // 弹窗默认显示「已选」，点「从上游拉取」后才出现「新获取」页签。
  const available = NO_MODELS;

  const { matches } = useVendorMatches(models);

  /** 已选模型里属于其他厂商的部分：保存时会被校验拒绝。 */
  const foreign = useMemo(() => {
    if (vendor === '') return NO_MODELS;
    return models.filter((m) => {
      const owner = matches.get(m)?.vendor;
      return owner !== undefined && owner !== '' && owner !== vendor;
    });
  }, [models, matches, vendor]);

  const description = useMemo(() => {
    const head = `${row?.name ?? ''}。一个模型名只能属于一个厂商分组，保存时会校验。`;
    if (vendor === '') return head;
    const hint = `当前渠道属于厂商「${vendor}」，其他厂商的模型会归到「其他厂商」组，保存会被校验拒绝。`;
    const warn = foreign.length > 0 ? `已选里还有其他厂商的模型：${foreign.join('、')}。` : '';
    return `${head}${hint}${warn}`;
  }, [row?.name, vendor, foreign]);

  return (
    <ModelPickerDialog
      open={row !== null}
      title="编辑模型"
      description={description}
      available={available}
      selected={models}
      vendorFilter={vendor === '' ? undefined : [vendor]}
      onFetch={linked ? async () => (await api.fetchUpstreamModels(upstreamId, tokenId, row?.type)).models : undefined}
      confirmText="保存到草稿"
      onConfirm={(next) => {
        if (row) onSave(row, next);
        onClose();
      }}
      onClose={onClose}
    />
  );
}
