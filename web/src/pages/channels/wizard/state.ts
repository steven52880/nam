import type { BatchRequest } from '@/api/types';

export interface WizardState {
  upstreamId: number | null;
  tokenIds: number[];
  /** 每个已选令牌要开放的模型：进入「厂商与模型」步骤时由上游拉取结果预选，只对本次创建生效。 */
  tokenModels: Record<number, string[]>;
  priority: string;
  weight: string;
  tag: string;
  testModels: Record<string, string>;
  discounts: Record<number, string>;
}

export const INITIAL_WIZARD: WizardState = {
  upstreamId: null,
  tokenIds: [],
  tokenModels: {},
  priority: '0',
  weight: '10',
  tag: '',
  testModels: {},
  discounts: {},
};

export const STEP_TITLES = ['选上游', '选令牌', '厂商与模型', '命名与默认值', '预览'] as const;

export function isInt(text: string, min?: number): boolean {
  return /^-?\d+$/.test(text.trim()) && (min === undefined || Number(text) >= min);
}

/** 所有已选令牌的模型合集（去重）。 */
export function allSelectedModels(state: WizardState): string[] {
  return [...new Set(state.tokenIds.flatMap((id) => state.tokenModels[id] ?? []))];
}

export function buildBatchRequest(state: WizardState): BatchRequest | null {
  if (state.upstreamId === null) return null;
  const discount: Record<string, number> = {};
  for (const [tokenId, text] of Object.entries(state.discounts)) {
    const value = Number(text);
    if (text.trim() !== '' && Number.isFinite(value) && value > 0) discount[tokenId] = value;
  }
  const testModel: Record<string, string> = {};
  for (const [vendor, model] of Object.entries(state.testModels)) {
    if (model.trim()) testModel[vendor] = model.trim();
  }
  const tokenModels: Record<string, string[]> = {};
  for (const id of state.tokenIds) tokenModels[String(id)] = state.tokenModels[id] ?? [];
  return {
    upstream_id: state.upstreamId,
    token_ids: state.tokenIds,
    token_models: tokenModels,
    priority: Number(state.priority),
    weight: Number(state.weight),
    tag: state.tag.trim(),
    test_model: testModel,
    discount,
  };
}
