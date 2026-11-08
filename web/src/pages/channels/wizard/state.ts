import type { BatchRequest } from '@/api/types';

export interface WizardState {
  upstreamId: number | null;
  tokenIds: number[];
  models: string[];
  priority: string;
  weight: string;
  tag: string;
  testModels: Record<string, string>;
  discounts: Record<number, string>;
}

export const INITIAL_WIZARD: WizardState = {
  upstreamId: null,
  tokenIds: [],
  models: [],
  priority: '0',
  weight: '10',
  tag: '',
  testModels: {},
  discounts: {},
};

export const STEP_TITLES = ['选上游', '选令牌', '选模型', '命名与默认值', '预览'] as const;

export function isInt(text: string, min?: number): boolean {
  return /^-?\d+$/.test(text.trim()) && (min === undefined || Number(text) >= min);
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
  return {
    upstream_id: state.upstreamId,
    token_ids: state.tokenIds,
    models: state.models,
    priority: Number(state.priority),
    weight: Number(state.weight),
    tag: state.tag.trim(),
    test_model: testModel,
    discount,
  };
}
