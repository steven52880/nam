import type { Credential, CredType, UpstreamInput, UpstreamTestInput, UpstreamType } from '@/api/types';

export interface UpstreamDraft {
  name: string;
  baseUrl: string;
  type: UpstreamType;
  credType: CredType;
  credential: Credential;
  userId: string;
  rate: string;
  tags: string;
  remark: string;
  syncNow: boolean;
}

export const INITIAL_DRAFT: UpstreamDraft = {
  name: '',
  baseUrl: '',
  type: 'newapi',
  credType: 'access_token',
  credential: {},
  userId: '',
  rate: '7',
  tags: '',
  remark: '',
  syncNow: true,
};

export const WIZARD_STEPS = ['名称和地址', '自动识别', '凭证方式', '连通性测试', '汇率与备注'] as const;

export const URL_PATTERN = /^https?:\/\/[^\s/]+/i;

export function credentialOk(draft: UpstreamDraft): boolean {
  const c = draft.credential;
  switch (draft.credType) {
    case 'access_token':
      return !!c.access_token?.trim() && /^\d+$/.test(draft.userId.trim());
    case 'password':
      return !!c.username?.trim() && !!c.password;
    case 'sk_key':
      return !!c.key?.trim();
    default:
      return true;
  }
}

export function toTestInput(draft: UpstreamDraft): UpstreamTestInput {
  return {
    type: draft.type,
    base_url: draft.baseUrl.trim(),
    cred_type: draft.credType,
    credential: draft.credential,
    platform_user_id: draft.credType === 'access_token' ? Number(draft.userId) : 0,
    adapter_config: '',
  };
}

export function toCreateInput(draft: UpstreamDraft): UpstreamInput {
  const body: UpstreamInput = {
    name: draft.name.trim(),
    type: draft.type,
    base_url: draft.baseUrl.trim(),
    cred_type: draft.credType,
    currency_rate: Number(draft.rate),
    tags: draft.tags.trim(),
    remark: draft.remark.trim(),
  };
  if (draft.credType) body.credential = draft.credential;
  if (draft.credType === 'access_token' && draft.userId) body.platform_user_id = Number(draft.userId);
  return body;
}
