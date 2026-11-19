import { ChevronDown, ChevronRight } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { ValidationIssue } from '@/api/types';
import { Banner } from '@/components/ui/banner';
import { groupIssues } from './issues';

interface ChannelIssuesBarProps {
  issues: readonly ValidationIssue[];
  /** 点击某个渠道：定位并选中它。 */
  onLocate: (channelId: number) => void;
}

/** 渠道表上方的规则问题提示条：没有问题不显示；有问题显示条数，可展开逐条看，点渠道名定位。 */
export function ChannelIssuesBar({ issues, onLocate }: ChannelIssuesBarProps) {
  const [open, setOpen] = useState(false);
  const groups = useMemo(() => groupIssues(issues), [issues]);
  if (issues.length === 0) return null;
  return (
    <Banner tone="warning">
      <button type="button" className="flex w-full items-center gap-1 text-left" onClick={() => setOpen(!open)}>
        {open ? <ChevronDown className="h-3.5 w-3.5 shrink-0" /> : <ChevronRight className="h-3.5 w-3.5 shrink-0" />}
        <span className="font-medium">渠道规则问题 {issues.length} 条</span>
        <span>（涉及 {groups.length} 个渠道）</span>
        <span className="ml-auto">{open ? '收起' : '展开'}</span>
      </button>
      {open && (
        <ul className="mt-1 max-h-40 overflow-y-auto">
          {groups.map((g) => (
            <li key={g.channelId} className="flex gap-2 border-t border-warning/30 py-0.5">
              <button
                type="button"
                className="w-48 shrink-0 truncate text-left font-medium underline-offset-2 hover:underline"
                title="点击定位到这个渠道"
                onClick={() => onLocate(g.channelId)}
              >
                {g.name || `渠道 ${g.channelId}`}
              </button>
              <span className="min-w-0 flex-1">{g.messages.join('；')}</span>
            </li>
          ))}
        </ul>
      )}
    </Banner>
  );
}
