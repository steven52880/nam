import { request } from './client';
import type {
  AuditLog,
  AuditQuery,
  BackendInfo,
  Page,
  PasswordInput,
  Session,
  Settings,
  Upstream,
  UpstreamInput,
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

  jobs: () => request<JobStatus[]>('/jobs'),
  runJob: (name: string) => request<null>(`/jobs/${encodeURIComponent(name)}`, { method: 'POST' }),

  settings: () => request<Settings>('/settings'),
  saveSettings: (body: Settings) => request<Settings>('/settings', { method: 'PUT', body }),

  audit: (q: AuditQuery) =>
    request<Page<AuditLog>>('/audit', { query: { ...q } }),
};
