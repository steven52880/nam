import type { AuditResult, AuditTrigger } from '@/api/types';
import { StatusBadge } from '@/components/cells/StatusBadge';
import { Badge } from '@/components/ui/misc';
import { AUDIT_RESULT } from '@/lib/labels';

export function AuditResultBadge({ result }: { result: AuditResult }) {
  return <StatusBadge dict={AUDIT_RESULT} value={result} />;
}

export function TriggerBadge({ trigger }: { trigger: AuditTrigger }) {
  return <Badge tone={trigger === 'auto' ? 'accent' : 'neutral'}>{trigger === 'auto' ? '自动' : '手动'}</Badge>;
}
