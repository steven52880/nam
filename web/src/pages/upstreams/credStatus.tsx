import type { Upstream } from '@/api/types';
import { formatAgo } from '@/lib/format';
import { cn } from '@/lib/utils';

const DAY = 86400;

export interface CredNoteInfo {
  text: string;
  tone: 'muted' | 'warning' | 'danger';
}

/** 凭证有效期 / 更新时间的一行提示；没有可显示的信息时返回 null。 */
export function credNote(u: Upstream, nowMs: number = Date.now()): CredNoteInfo | null {
  if (u.cred_type === 'cookie' && u.cred_expires_at > 0) {
    const left = u.cred_expires_at - Math.floor(nowMs / 1000);
    if (left <= 0) return { text: 'Cookie 已过期', tone: 'danger' };
    return { text: `Cookie 约 ${Math.ceil(left / DAY)} 天后过期`, tone: left <= 3 * DAY ? 'warning' : 'muted' };
  }
  if (u.cred_type === 'sub2api_session' && u.cred_updated_at > 0) {
    return { text: `凭证更新于 ${formatAgo(u.cred_updated_at, nowMs)}`, tone: 'muted' };
  }
  return null;
}

export function isCredInvalid(u: Upstream): boolean {
  return u.last_sync_error.includes('已失效');
}

const TONE_CLASS: Record<CredNoteInfo['tone'], string> = {
  muted: 'text-muted-foreground',
  warning: 'text-warning',
  danger: 'text-destructive',
};

export function CredNote({ u, className }: { u: Upstream; className?: string }) {
  const note = credNote(u);
  if (!note) return null;
  return <span className={cn('text-xs', TONE_CLASS[note.tone], className)}>{note.text}</span>;
}
