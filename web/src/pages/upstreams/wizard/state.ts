import type { Credential, CredType, UpstreamInput, UpstreamTestInput, UpstreamType } from '@/api/types';
import { allowsCredType, defaultCredType } from '../constants';

export interface UpstreamDraft {
  name: string;
  baseUrl: string;
  type: UpstreamType;
  credType: CredType;
  credential: Credential;
  userId: string;
  loginCaptcha: boolean;
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
  loginCaptcha: false,
  rate: '7',
  tags: '',
  remark: '',
  syncNow: true,
};

export const WIZARD_STEPS = ['名称和地址', '自动识别', '凭证方式', '连通性测试', '汇率与备注'] as const;

export const URL_PATTERN = /^https?:\/\/[^\s/]+/i;

export function credentialValid(type: UpstreamType, credType: CredType, c: Credential, userId: string): boolean {
  const uid = userId.trim();
  switch (credType) {
    case 'access_token':
      return !!c.access_token?.trim() && /^\d+$/.test(uid);
    case 'password':
      return !!c.username?.trim() && !!c.password;
    case 'sk_key':
      return !!c.key?.trim();
    case 'cookie':
      return !!c.cookie?.trim() && (type === 'newapi' ? /^\d+$/.test(uid) : uid === '' || /^\d+$/.test(uid));
    case 'sub2api_session':
      return !!c.access_token?.trim() && !!c.refresh_token?.trim();
    default:
      return true;
  }
}

export function credentialOk(draft: UpstreamDraft): boolean {
  return credentialValid(draft.type, draft.credType, draft.credential, draft.userId);
}

/**
 * 上游类型或探测结果变化后，调整凭证方式：
 * 还没填凭证时回到该类型的默认值（登录要人机验证的 NewAPI / OneHub 默认 cookie）；
 * 已填但当前类型不支持时，清空并回到默认值。
 */
export function credPatchForType(
  type: UpstreamType,
  credType: CredType,
  credential: Credential,
  loginCaptcha: boolean,
): Partial<UpstreamDraft> {
  const captchaSite = loginCaptcha && (type === 'newapi' || type === 'onehub');
  const next: CredType = captchaSite ? 'cookie' : defaultCredType(type);
  const untouched = Object.values(credential).every((v) => !v);
  if (untouched) return next === credType ? {} : { credType: next, credential: {} };
  if (!allowsCredType(type, credType)) return { credType: next, credential: {} };
  return {};
}

/** 需要同时提交平台用户 ID 的凭证方式。 */
export function usesUserId(credType: CredType): boolean {
  return credType === 'access_token' || credType === 'cookie';
}

export function toTestInput(draft: UpstreamDraft): UpstreamTestInput {
  return {
    type: draft.type,
    base_url: draft.baseUrl.trim(),
    cred_type: draft.credType,
    credential: draft.credential,
    platform_user_id: usesUserId(draft.credType) ? Number(draft.userId) || 0 : 0,
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
  if (usesUserId(draft.credType) && draft.userId) body.platform_user_id = Number(draft.userId);
  return body;
}
