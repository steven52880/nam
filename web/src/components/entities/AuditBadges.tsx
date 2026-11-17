import type { AuditResult, AuditTrigger } from '@/api/types';
import { Badge } from '@/components/ui/misc';

export function AuditResultBadge({ result }: { result: AuditResult }) {
  if (result === 'ok') return <Badge tone="success">成功</Badge>;
  if (result === 'fail') return <Badge tone="danger">失败</Badge>;
  return <Badge>已跳过</Badge>;
}

export function TriggerBadge({ trigger }: { trigger: AuditTrigger }) {
  return <Badge tone={trigger === 'auto' ? 'accent' : 'neutral'}>{trigger === 'auto' ? '自动' : '手动'}</Badge>;
}
