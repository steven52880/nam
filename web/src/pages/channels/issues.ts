import type { ValidationIssue } from '@/api/types';

export interface ChannelIssueGroup {
  channelId: number;
  name: string;
  messages: string[];
}

/** 规则问题按渠道归并：保持首次出现的顺序，同一渠道的多条问题放在一起。 */
export function groupIssues(items: readonly ValidationIssue[]): ChannelIssueGroup[] {
  const groups = new Map<number, ChannelIssueGroup>();
  for (const it of items) {
    const g = groups.get(it.channel_id);
    if (g) g.messages.push(it.message);
    else groups.set(it.channel_id, { channelId: it.channel_id, name: it.name, messages: [it.message] });
  }
  return [...groups.values()];
}
