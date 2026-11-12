import { request } from './client';
import type {
  AliasEntry,
  ApplyResponse,
  AuditLog,
  AuditQuery,
  BackendInfo,
  BatchPreview,
  BatchRequest,
  BatchSpec,
  CacheOverrideInput,
  ChannelChange,
  ChannelList,
  ChannelModels,
  ChannelTestResult,
  CheckResult,
  DeriveChannel,
  DriftDetail,
  MatchTokensRequest,
  MatchTokensResponse,
  ModelDiscountInput,
  PriceDetail,
  PriceList,
  PriceOverride,
  PriceQuery,
  PriceSyncResult,
  IssuesPayload,
  Page,
  PasswordInput,
  PlanItem,
  ProbeResult,
  RemoteTokenInput,
  Session,
  Settings,
  SyncResult,
  TokenInput,
  Upstream,
  UpstreamGroup,
  UpstreamInput,
  UpstreamModel,
  UpstreamTestInput,
  UpstreamToken,
  Vendor,
  VendorInput,
  JobStatus,
} from './types';

export const api = {
  session: () => request<Session>('/session', { silent: true }),
  backend: () => request<BackendInfo>('/backend'),

  login: (password: string) =>
    request<null>('/auth/login', { method: 'POST', body: { password }, silent: true }),
  logout: () => request<null>('/auth/logout', { method: 'POST' }),
  changePassword: (body: PasswordInput) => request<null>('/auth/password', { method: 'POST', body }),

  upstreams: () => request<Upstream[]>('/upstreams'),
  createUpstream: (body: UpstreamInput) => request<Upstream>('/upstreams', { method: 'POST', body }),
  updateUpstream: (id: number, body: UpstreamInput) =>
    request<Upstream>(`/upstreams/${id}`, { method: 'PUT', body }),
  deleteUpstream: (id: number) => request<null>(`/upstreams/${id}`, { method: 'DELETE' }),

  probeUpstream: (baseUrl: string) =>
    request<ProbeResult>('/upstreams/probe', { method: 'POST', body: { base_url: baseUrl } }),
  testUpstreamInput: (body: UpstreamTestInput) =>
    request<CheckResult[]>('/upstreams/test', { method: 'POST', body }),
  testUpstream: (id: number) => request<CheckResult[]>(`/upstreams/${id}/test`, { method: 'POST' }),
  syncUpstream: (id: number, what: string[]) =>
    request<SyncResult[]>(`/upstreams/${id}/sync`, { method: 'POST', body: { what } }),
  upstreamTokens: (id: number) => request<UpstreamToken[]>(`/upstreams/${id}/tokens`),
  createToken: (id: number, body: TokenInput) =>
    request<UpstreamToken>(`/upstreams/${id}/tokens`, { method: 'POST', body }),
  createRemoteToken: (id: number, body: RemoteTokenInput) =>
    request<UpstreamToken>(`/upstreams/${id}/tokens/remote`, { method: 'POST', body }),
  deleteToken: (id: number, tokenId: number) =>
    request<null>(`/upstreams/${id}/tokens/${tokenId}`, { method: 'DELETE' }),
  upstreamGroups: (id: number) => request<UpstreamGroup[]>(`/upstreams/${id}/groups`),
  upstreamModels: (id: number) => request<UpstreamModel[]>(`/upstreams/${id}/models`),
  setTokenModels: (id: number, tokenId: number, models: string[]) =>
    request<UpstreamToken>(`/upstreams/${id}/tokens/${tokenId}/models`, { method: 'PUT', body: { models } }),
  fetchUpstreamModels: (id: number, tokenId: number, channelType?: number) =>
    request<{ models: string[] }>(`/upstreams/${id}/models/fetch`, {
      method: 'POST',
      body: { token_id: tokenId, channel_type: channelType },
    }),

  channels: () => request<ChannelList>('/channels'),
  patchDrafts: (changes: ChannelChange[]) =>
    request<IssuesPayload>('/channels/drafts', { method: 'PATCH', body: { changes } }),
  discardDrafts: (ids?: number[]) =>
    request<null>('/channels/drafts', { method: 'DELETE', body: ids ? { ids } : {} }),
  batchPreview: (body: BatchRequest) =>
    request<BatchPreview>('/channels/batch/preview', { method: 'POST', body }),
  matchTokens: (body: MatchTokensRequest) =>
    request<MatchTokensResponse>('/channels/match-tokens', { method: 'POST', body }),
  batchCreate: (specs: BatchSpec[]) =>
    request<{ ids: number[] }>('/channels/batch', { method: 'POST', body: { specs } }),
  deleteChannels: (ids: number[]) => request<null>('/channels/delete', { method: 'POST', body: { ids } }),
  plan: (ids?: number[]) =>
    request<PlanItem[]>('/channels/plan', { query: { ids: ids?.join(',') } }),
  apply: (ids?: number[]) =>
    request<ApplyResponse>('/channels/apply', { method: 'POST', body: ids ? { ids } : {} }),
  drift: (id: number) => request<DriftDetail>(`/channels/${id}/drift`),
  acceptDrift: (id: number) => request<null>(`/channels/${id}/drift/accept`, { method: 'POST' }),
  overwriteDrift: (id: number) => request<null>(`/channels/${id}/drift/overwrite`, { method: 'POST' }),
  testChannel: (id: number, model: string) =>
    request<ChannelTestResult>(`/channels/${id}/test`, { method: 'POST', body: { model } }),
  newapiLink: (id: number) => request<{ url: string }>(`/channels/${id}/newapi-link`),

  channelModels: (id: number) => request<ChannelModels>(`/channels/${id}/models`),
  setModelDiscount: (id: number, body: ModelDiscountInput) =>
    request<null>(`/channels/${id}/model-discount`, { method: 'PUT', body }),
  setCacheOverride: (id: number, body: CacheOverrideInput) =>
    request<null>(`/channels/${id}/cache-override`, { method: 'PUT', body }),

  prices: (query: PriceQuery) => request<PriceList>('/prices', { query: { ...query } }),
  priceDetail: (model: string) => request<PriceDetail>(`/prices/${encodeURIComponent(model)}`),
  setPriceOverride: (model: string, body: PriceOverride) =>
    request<null>(`/prices/${encodeURIComponent(model)}/override`, { method: 'PUT', body }),
  clearPriceOverride: (model: string) =>
    request<null>(`/prices/${encodeURIComponent(model)}/override`, { method: 'DELETE' }),
  syncPrices: () => request<PriceSyncResult>('/prices/sync', { method: 'POST' }),
  aliases: () => request<AliasEntry[]>('/aliases'),
  setAliases: (official: string, aliases: string[]) =>
    request<null>(`/aliases/${encodeURIComponent(official)}`, { method: 'PUT', body: { aliases } }),
  deriveUpstream: (id: number) => request<DeriveChannel[]>(`/upstreams/${id}/derive`, { method: 'POST' }),

  vendors: () => request<Vendor[]>('/vendors'),
  createVendor: (body: VendorInput) => request<Vendor>('/vendors', { method: 'POST', body }),
  updateVendor: (id: number, body: VendorInput) => request<Vendor>(`/vendors/${id}`, { method: 'PUT', body }),
  deleteVendor: (id: number) => request<null>(`/vendors/${id}`, { method: 'DELETE' }),
  reorderVendors: (ids: number[]) => request<null>('/vendors/order', { method: 'PUT', body: { ids } }),

  jobs: () => request<JobStatus[]>('/jobs'),
  runJob: (name: string) => request<null>(`/jobs/${encodeURIComponent(name)}`, { method: 'POST' }),

  settings: () => request<Settings>('/settings'),
  saveSettings: (body: Settings) => request<Settings>('/settings', { method: 'PUT', body }),

  audit: (q: AuditQuery) =>
    request<Page<AuditLog>>('/audit', { query: { ...q } }),
};
