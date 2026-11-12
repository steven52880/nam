import type { AuditResult, AuditTrigger } from '@/api/types';
import { Badge } from '@/components/ui/misc';

export const KIND_LABELS: Record<string, string> = {
  channel: '渠道',
  price: '价格',
  setting: '设置',
  upstream: '上游',
  cache_test: '缓存测试',
};

export function AuditResultBadge({ result }: { result: AuditResult }) {
  if (result === 'ok') return <Badge tone="success">成功</Badge>;
  if (result === 'fail') return <Badge tone="danger">失败</Badge>;
  return <Badge>已跳过</Badge>;
}

export function TriggerBadge({ trigger }: { trigger: AuditTrigger }) {
  return <Badge tone={trigger === 'auto' ? 'accent' : 'neutral'}>{trigger === 'auto' ? '自动' : '手动'}</Badge>;
}
